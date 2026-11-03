package auth

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

func (s *Service) verifyBearer(ctx context.Context, token string) (Claims, User, error) {
	claims, err := VerifyToken(s.privateKey.Public().(ed25519.PublicKey), token, s.now(), AudienceAPI)
	if err != nil {
		return Claims{}, User{}, ErrUnauthenticated
	}
	user, err := s.repo.UserByID(ctx, claims.Subject)
	if errors.Is(err, ErrUserNotFound) {
		return Claims{}, User{}, ErrUnauthenticated
	}
	if err != nil {
		return Claims{}, User{}, err
	}
	return claims, user, nil
}

func (s *Service) ListMemberships(ctx context.Context, token string) ([]TenantMembership, error) {
	_, user, err := s.verifyBearer(ctx, token)
	if err != nil {
		return nil, err
	}
	return s.repo.ListUserMemberships(ctx, user.ID)
}

func (s *Service) ExchangeTenantSession(ctx context.Context, token, tenantID string) (TenantSession, error) {
	_, user, err := s.verifyBearer(ctx, token)
	if err != nil {
		return TenantSession{}, err
	}
	parsedTenant, err := uuid.Parse(tenantID)
	if err != nil || parsedTenant.String() != tenantID {
		return TenantSession{}, fmt.Errorf("%w: tenant_id must be a canonical UUID", ErrInvalidInput)
	}
	membership, err := s.repo.ActiveMembership(ctx, user.ID, tenantID)
	if errors.Is(err, ErrMembershipNotFound) {
		return TenantSession{}, ErrForbidden
	}
	if err != nil {
		return TenantSession{}, err
	}
	now := s.now().UTC()
	signed, expiresAt, err := SignTenantToken(s.privateKey, user.ID, tenantID, membership.PermissionProfile, now, s.ttl)
	if err != nil {
		return TenantSession{}, err
	}
	return TenantSession{
		AccessToken:       signed,
		TokenType:         "Bearer",
		ExpiresAt:         expiresAt,
		TenantID:          tenantID,
		PermissionProfile: membership.PermissionProfile,
	}, nil
}

func (s *Service) requireTenantAccess(ctx context.Context, token, tenantID string, requireOwner bool) (User, TenantMembership, error) {
	_, user, err := s.verifyBearer(ctx, token)
	if err != nil {
		return User{}, TenantMembership{}, err
	}
	parsedTenant, err := uuid.Parse(tenantID)
	if err != nil || parsedTenant.String() != tenantID {
		return User{}, TenantMembership{}, fmt.Errorf("%w: tenant_id must be a canonical UUID", ErrInvalidInput)
	}
	membership, err := s.repo.ActiveMembership(ctx, user.ID, tenantID)
	if errors.Is(err, ErrMembershipNotFound) {
		return User{}, TenantMembership{}, ErrForbidden
	}
	if err != nil {
		return User{}, TenantMembership{}, err
	}
	if requireOwner && membership.PermissionProfile != ProfileTenantOwner {
		return User{}, TenantMembership{}, ErrForbidden
	}
	return user, membership, nil
}

func (s *Service) ListTenantMembers(ctx context.Context, token, tenantID string) ([]TenantMembership, error) {
	if _, _, err := s.requireTenantAccess(ctx, token, tenantID, false); err != nil {
		return nil, err
	}
	return s.repo.ListTenantMembers(ctx, tenantID)
}

func (s *Service) InviteTenantMember(ctx context.Context, token, tenantID string, input InviteMemberInput) (InviteMemberResult, error) {
	user, _, err := s.requireTenantAccess(ctx, token, tenantID, true)
	if err != nil {
		return InviteMemberResult{}, err
	}
	email, err := normalizeEmail(input.Email)
	if err != nil {
		return InviteMemberResult{}, err
	}
	profile := strings.TrimSpace(input.PermissionProfile)
	if profile == "" {
		profile = ProfileTenantViewer
	}
	if profile != ProfileTenantViewer && profile != ProfileTenantOwner {
		return InviteMemberResult{}, fmt.Errorf("%w: invalid permission_profile", ErrInvalidInput)
	}
	input.TenantID = tenantID
	input.ActorID = user.ID
	input.Email = email
	input.PermissionProfile = profile
	return s.repo.InviteTenantMember(ctx, input)
}

func (s *Service) RevokeTenantMember(ctx context.Context, token, tenantID, membershipID string) error {
	user, callerMembership, err := s.requireTenantAccess(ctx, token, tenantID, true)
	if err != nil {
		return err
	}
	parsedMembership, err := uuid.Parse(membershipID)
	if err != nil || parsedMembership.String() != membershipID {
		return fmt.Errorf("%w: membership_id must be a canonical UUID", ErrInvalidInput)
	}
	if callerMembership.MembershipID == membershipID {
		return fmt.Errorf("%w: owner cannot revoke own membership", ErrProvisionConflict)
	}
	return s.repo.RevokeTenantMember(ctx, tenantID, membershipID, user.ID)
}
