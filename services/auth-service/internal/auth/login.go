package auth

import (
	"context"
	"crypto/ed25519"
	"errors"

	"golang.org/x/crypto/bcrypt"
)

func (s *Service) Login(ctx context.Context, email, password string) (Session, error) {
	email, err := normalizeEmail(email)
	if err != nil || password == "" {
		return Session{}, ErrInvalidCredentials
	}
	user, err := s.repo.UserByEmail(ctx, email)
	if errors.Is(err, ErrInvalidCredentials) || (err == nil && bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil) {
		return Session{}, ErrInvalidCredentials
	}
	if err != nil {
		return Session{}, err
	}
	return s.newSession(ctx, user.User)
}

func (s *Service) Me(ctx context.Context, token string) (User, error) {
	claims, err := VerifyToken(s.privateKey.Public().(ed25519.PublicKey), token, s.now(), AudienceAPI)
	if err != nil {
		return User{}, ErrUnauthenticated
	}
	user, err := s.repo.UserByID(ctx, claims.Subject)
	if errors.Is(err, ErrUserNotFound) {
		return User{}, ErrUnauthenticated
	}
	if err != nil {
		return User{}, err
	}
	return user, nil
}
