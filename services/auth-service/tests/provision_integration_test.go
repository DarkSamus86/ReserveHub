package tests

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"darksamus.com/ReserveHub/auth-service/internal/auth"
	"darksamus.com/ReserveHub/auth-service/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestProvisionOwnerIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	store, err := repository.NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	service := auth.NewService(store, signingKey(t), 15*time.Minute)

	// Tenant accepts slugs that begin with a digit and contain up to 100 bytes.
	slug := "7" + strings.Repeat("a", 99)
	input := auth.ProvisionOwnerInput{TenantID: uuid.NewString(), OperationID: uuid.NewString(), Slug: slug, Email: "new-" + uuid.NewString() + "@example.com"}
	first, err := service.ProvisionOwner(ctx, input)
	if err != nil || !first.Created || first.Status != "pending_delivery" {
		t.Fatalf("first provision: %+v, %v", first, err)
	}
	replayed, err := service.ProvisionOwner(ctx, input)
	if err != nil || replayed.Created || replayed.UserID != first.UserID || replayed.MembershipID != first.MembershipID || replayed.InvitationID != first.InvitationID {
		t.Fatalf("replayed provision: %+v, %v", replayed, err)
	}
	conflict := input
	conflict.Email = "different-" + uuid.NewString() + "@example.com"
	if _, err := service.ProvisionOwner(ctx, conflict); !errors.Is(err, auth.ErrProvisionConflict) {
		t.Fatalf("changed payload: %v", err)
	}
	var roleCode, membershipStatus, invitationStatus, userStatus string
	var passwordHash *string
	err = pool.QueryRow(ctx, `SELECT r.code, m.status, i.status, u.status, u.password_hash
		FROM auth_service.ln_user_tenant_membership m
		JOIN auth_service.md_role r ON r.id=m.tenant_role_id
		JOIN auth_service.md_user u ON u.id=m.user_id
		JOIN auth_service.tr_tenant_invitation i ON i.membership_id=m.id
		WHERE m.id=$1`, first.MembershipID).Scan(&roleCode, &membershipStatus, &invitationStatus, &userStatus, &passwordHash)
	if err != nil {
		t.Fatal(err)
	}
	if roleCode != slug || membershipStatus != "PENDING" || invitationStatus != "PENDING_DELIVERY" || userStatus != "INVITED" || passwordHash != nil {
		t.Fatalf("unexpected provisioned state: role=%q membership=%q invitation=%q user=%q password=%v", roleCode, membershipStatus, invitationStatus, userStatus, passwordHash)
	}
	if _, err := service.Login(ctx, input.Email, "any password at all"); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("invited user login: %v", err)
	}

	existingEmail := "existing-" + uuid.NewString() + "@example.com"
	if _, err := service.Register(ctx, existingEmail, "a long password 123"); err != nil {
		t.Fatal(err)
	}
	existing := auth.ProvisionOwnerInput{TenantID: uuid.NewString(), OperationID: uuid.NewString(), Slug: "existing-owner", Email: existingEmail}
	existingResult, err := service.ProvisionOwner(ctx, existing)
	if err != nil {
		t.Fatal(err)
	}
	err = pool.QueryRow(ctx, `SELECT m.status, u.status FROM auth_service.ln_user_tenant_membership m JOIN auth_service.md_user u ON u.id=m.user_id WHERE m.id=$1`, existingResult.MembershipID).Scan(&membershipStatus, &userStatus)
	if err != nil {
		t.Fatal(err)
	}
	if membershipStatus != "PENDING" || userStatus != "ACTIVE" {
		t.Fatalf("existing owner state: membership=%s user=%s", membershipStatus, userStatus)
	}
}
