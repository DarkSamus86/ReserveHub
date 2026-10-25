package tests

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"darksamus.com/ReserveHub/auth-service/internal/auth"
)

func TestTokenClaimsAndExpiry(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	token, expiresAt, err := auth.SignToken(privateKey, "user-123", auth.RoleAdmin, "platform", now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := auth.VerifyToken(publicKey, token, now, auth.AudienceAPI)
	if err != nil || claims.Role != auth.RoleAdmin || claims.Scope != "platform" || expiresAt.Unix() != claims.Expires {
		t.Fatalf("claims = %+v, expiry = %v, err = %v", claims, expiresAt, err)
	}
	if _, err := auth.VerifyToken(publicKey, token, now.Add(time.Minute), auth.AudienceAPI); err == nil {
		t.Fatal("expired token was accepted")
	}
	if _, err := auth.VerifyToken(publicKey, token, now, auth.AudienceTenant); err != nil {
		t.Fatalf("tenant audience token rejected: %v", err)
	}
}

func TestTokenRejectsTamperingAndInvalidSigningKey(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	token, _, err := auth.SignToken(privateKey, "user-123", auth.RoleUser, "platform", now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := auth.VerifyToken(publicKey, token+"x", now, auth.AudienceAPI); err == nil {
		t.Fatal("tampered token was accepted")
	}
	if _, _, err := auth.SignToken(ed25519.PrivateKey("short"), "user-123", auth.RoleUser, "platform", now, time.Minute); err == nil {
		t.Fatal("invalid signing key was accepted")
	}
}
