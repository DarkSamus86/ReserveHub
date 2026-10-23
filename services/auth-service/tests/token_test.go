package tests

import (
	"testing"
	"time"

	"darksamus.com/ReserveHub/auth-service/internal/auth"
)

func TestTokenRoundTripAndExpiry(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	now := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	token, _, err := auth.SignToken(secret, "user-123", auth.RoleAdmin, now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := auth.VerifyToken(secret, token, now)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Subject != "user-123" || claims.Role != auth.RoleAdmin {
		t.Fatalf("unexpected claims: %+v", claims)
	}
	if _, err := auth.VerifyToken(secret, token, now.Add(time.Minute)); err == nil {
		t.Fatal("expired token was accepted")
	}
}

func TestTokenRejectsTamperingAndShortSecret(t *testing.T) {
	secret := []byte("0123456789abcdef0123456789abcdef")
	now := time.Now().UTC()
	token, _, err := auth.SignToken(secret, "user-123", auth.RoleUser, now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := auth.VerifyToken(secret, token+"x", now); err == nil {
		t.Fatal("tampered token was accepted")
	}
	if _, _, err := auth.SignToken([]byte("short"), "user-123", auth.RoleUser, now, time.Minute); err == nil {
		t.Fatal("short signing secret was accepted")
	}
}
