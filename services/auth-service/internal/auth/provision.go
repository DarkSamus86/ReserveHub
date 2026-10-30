package auth

import (
	"context"
	"fmt"
	"regexp"

	"github.com/google/uuid"
)

var tenantSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type ProvisionOwnerInput struct {
	TenantID    string `json:"tenant_id"`
	Slug        string `json:"slug"`
	Email       string `json:"email"`
	OperationID string `json:"operation_id"`
}

type ProvisionOwnerResult struct {
	TenantID     string `json:"tenant_id"`
	UserID       string `json:"user_id"`
	MembershipID string `json:"membership_id"`
	InvitationID string `json:"invitation_id"`
	Status       string `json:"status"`
	Created      bool   `json:"-"`
}

func (s *Service) ProvisionOwner(ctx context.Context, input ProvisionOwnerInput) (ProvisionOwnerResult, error) {
	if id, err := uuid.Parse(input.TenantID); err != nil || id.String() != input.TenantID {
		return ProvisionOwnerResult{}, fmt.Errorf("%w: tenant_id must be a canonical UUID", ErrInvalidInput)
	}
	if id, err := uuid.Parse(input.OperationID); err != nil || id.String() != input.OperationID {
		return ProvisionOwnerResult{}, fmt.Errorf("%w: operation_id must be a canonical UUID", ErrInvalidInput)
	}
	if len(input.Slug) == 0 || len(input.Slug) > 100 || !tenantSlugPattern.MatchString(input.Slug) {
		return ProvisionOwnerResult{}, fmt.Errorf("%w: slug is invalid", ErrInvalidInput)
	}
	email, err := normalizeEmail(input.Email)
	if err != nil {
		return ProvisionOwnerResult{}, err
	}
	input.Email = email
	return s.repo.ProvisionOwner(ctx, input)
}
