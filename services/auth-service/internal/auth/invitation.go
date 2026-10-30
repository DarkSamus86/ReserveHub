package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type AcceptInvitationResult struct {
	TenantID     string `json:"tenant_id"`
	MembershipID string `json:"membership_id"`
	Status       string `json:"status"`
}

type InvitationStatus struct {
	InvitationID string     `json:"invitation_id"`
	Status       string     `json:"status"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
}

func (s *Service) InvitationStatus(ctx context.Context, id string) (InvitationStatus, error) {
	parsed, err := uuid.Parse(id)
	if err != nil || parsed.String() != id {
		return InvitationStatus{}, ErrInvalidInput
	}
	return s.repo.InvitationStatus(ctx, id)
}

func (s *Service) ReissueInvitation(ctx context.Context, id string) error {
	parsed, err := uuid.Parse(id)
	if err != nil || parsed.String() != id {
		return ErrInvalidInput
	}
	return s.repo.ReissueInvitation(ctx, id)
}

func (s *Service) AcceptInvitation(ctx context.Context, token, password string) (AcceptInvitationResult, error) {
	if len(token) != 43 || strings.ContainsAny(token, " \r\n\t") {
		return AcceptInvitationResult{}, ErrInvalidSecret
	}
	secret, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(secret) != 32 {
		return AcceptInvitationResult{}, ErrInvalidSecret
	}
	var hashPassword string
	if password != "" {
		if err = validatePassword(password); err != nil {
			return AcceptInvitationResult{}, err
		}
		hash, hashErr := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if hashErr != nil {
			return AcceptInvitationResult{}, fmt.Errorf("hash invitation password: %w", hashErr)
		}
		hashPassword = string(hash)
	}
	hash := sha256.Sum256([]byte(token))
	return s.repo.AcceptInvitation(ctx, hash[:], hashPassword)
}
