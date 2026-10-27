package auth

import (
	"context"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

func (s *Service) BootstrapAdmin(ctx context.Context, email, password string) error {
	hasAdmin, err := s.repo.HasAdmin(ctx)
	if err != nil {
		return fmt.Errorf("check for administrator: %w", err)
	}
	if hasAdmin {
		return nil
	}
	if email == "" || password == "" {
		return fmt.Errorf("no administrator exists; set AUTH_BOOTSTRAP_ADMIN_EMAIL and AUTH_BOOTSTRAP_ADMIN_PASSWORD for initial setup")
	}
	email, err = normalizeEmail(email)
	if err != nil {
		return err
	}
	if err := validatePassword(password); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash bootstrap admin password: %w", err)
	}
	return s.repo.BootstrapAdmin(ctx, email, string(hash))
}
