package auth

import (
	"context"
	"fmt"
	"net/mail"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

func (s *Service) Register(ctx context.Context, email, password string) (Session, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return Session{}, err
	}
	if err := validatePassword(password); err != nil {
		return Session{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return Session{}, fmt.Errorf("hash password: %w", err)
	}
	user, err := s.repo.CreateUser(ctx, email, string(hash), RoleUser)
	if err != nil {
		return Session{}, err
	}
	return s.newSession(ctx, user)
}

func normalizeEmail(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	parsed, err := mail.ParseAddress(value)
	if err != nil || parsed.Address != value || len(value) > 254 {
		return "", fmt.Errorf("%w: email is invalid", ErrInvalidInput)
	}
	return value, nil
}

func validatePassword(password string) error {
	if len(password) < 12 || len(password) > 72 {
		return fmt.Errorf("%w: password must be between 12 and 72 bytes", ErrInvalidInput)
	}
	return nil
}
