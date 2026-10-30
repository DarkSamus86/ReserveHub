package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"strings"

	"github.com/google/uuid"
)

func newRefreshSecret() (string, error) {
	var secret [32]byte
	if _, err := rand.Read(secret[:]); err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(secret[:])
	return encoded, nil
}

func parseRefreshToken(token string) (string, []byte, error) {
	if len(token) != 36+1+43 {
		return "", nil, ErrInvalidSecret
	}
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return "", nil, ErrInvalidSecret
	}
	id, err := uuid.Parse(parts[0])
	if err != nil || id.String() != parts[0] {
		return "", nil, ErrInvalidSecret
	}
	secret, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || len(secret) != 32 {
		return "", nil, ErrInvalidSecret
	}
	hash := sha256.Sum256([]byte(token))
	return parts[0], hash[:], nil
}

func (s *Service) Refresh(ctx context.Context, token string) (Session, error) {
	id, oldHash, err := parseRefreshToken(token)
	if err != nil {
		return Session{}, err
	}
	newSecret, err := newRefreshSecret()
	if err != nil {
		return Session{}, err
	}
	newToken := id + "." + newSecret
	newHash := sha256.Sum256([]byte(newToken))
	now := s.now().UTC()
	user, refreshExpiresAt, err := s.repo.RotateSession(ctx, id, oldHash, newHash[:], now)
	if err != nil {
		return Session{}, err
	}
	accessToken, expiresAt, err := SignToken(s.privateKey, user.ID, user.Role, user.Scope, now, s.ttl)
	if err != nil {
		return Session{}, err
	}
	return Session{AccessToken: accessToken, RefreshToken: newToken, TokenType: "Bearer", ExpiresAt: expiresAt, RefreshExpiresAt: refreshExpiresAt, User: user}, nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	id, hash, err := parseRefreshToken(token)
	if err != nil {
		return err
	}
	return s.repo.RevokeSession(ctx, id, hash)
}
