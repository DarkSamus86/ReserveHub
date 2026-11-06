package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"
)

func TestTokenClaimsAndExpiry(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	token, expiresAt, err := SignToken(privateKey, "user-123", RoleAdmin, "platform", now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := VerifyToken(publicKey, token, now, AudienceAPI)
	if err != nil || claims.Role != RoleAdmin || claims.Scope != "platform" || expiresAt.Unix() != claims.Expires {
		t.Fatalf("claims = %+v, expiry = %v, err = %v", claims, expiresAt, err)
	}
	if _, err := VerifyToken(publicKey, token, now.Add(time.Minute), AudienceAPI); err == nil {
		t.Fatal("expired token was accepted")
	}
	if _, err := VerifyToken(publicKey, token, now, AudienceTenant); err != nil {
		t.Fatalf("tenant audience token rejected: %v", err)
	}
}

func TestTokenRejectsTamperingAndInvalidSigningKey(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	token, _, err := SignToken(privateKey, "user-123", RoleUser, "platform", now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyToken(publicKey, token+"x", now, AudienceAPI); err == nil {
		t.Fatal("tampered token was accepted")
	}
	if _, _, err := SignToken(ed25519.PrivateKey("short"), "user-123", RoleUser, "platform", now, time.Minute); err == nil {
		t.Fatal("invalid signing key was accepted")
	}
}
