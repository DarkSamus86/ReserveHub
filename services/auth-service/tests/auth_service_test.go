package tests

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"testing"
	"time"

	"darksamus.com/ReserveHub/auth-service/internal/auth"
	"golang.org/x/crypto/bcrypt"
)

type memoryRepository struct {
	user             auth.User
	passwordHash     string
	created          bool
	provisionInput   auth.ProvisionOwnerInput
	provisionResult  auth.ProvisionOwnerResult
	provisionErr     error
	provisionCalls   int
	memberships      []auth.TenantMembership
	activeMembership auth.TenantMembership
	activeErr        error
	inviteResult     auth.InviteMemberResult
	inviteErr        error
	revokeErr        error
}

func (r *memoryRepository) CreateSession(context.Context, string, string, []byte, time.Time) error {
	return nil
}
func (r *memoryRepository) RotateSession(context.Context, string, []byte, []byte, time.Time) (auth.User, time.Time, error) {
	return auth.User{}, time.Time{}, auth.ErrInvalidSecret
}
func (r *memoryRepository) RevokeSession(context.Context, string, []byte) error {
	return auth.ErrInvalidSecret
}
func (r *memoryRepository) AcceptInvitation(context.Context, []byte, string) (auth.AcceptInvitationResult, error) {
	return auth.AcceptInvitationResult{}, auth.ErrInvalidSecret
}
func (r *memoryRepository) ReissueInvitation(context.Context, string) error {
	return auth.ErrInvalidSecret
}
func (r *memoryRepository) InvitationStatus(context.Context, string) (auth.InvitationStatus, error) {
	return auth.InvitationStatus{}, auth.ErrInvitationNotFound
}
func (r *memoryRepository) ListUserMemberships(context.Context, string) ([]auth.TenantMembership, error) {
	return r.memberships, nil
}
func (r *memoryRepository) ActiveMembership(_ context.Context, userID, tenantID string) (auth.TenantMembership, error) {
	if r.activeErr != nil {
		return auth.TenantMembership{}, r.activeErr
	}
	if r.activeMembership.TenantID == tenantID && (r.activeMembership.UserID == "" || r.activeMembership.UserID == userID) {
		return r.activeMembership, nil
	}
	return auth.TenantMembership{}, auth.ErrMembershipNotFound
}
func (r *memoryRepository) ListTenantMembers(context.Context, string) ([]auth.TenantMembership, error) {
	return r.memberships, nil
}
func (r *memoryRepository) InviteTenantMember(_ context.Context, input auth.InviteMemberInput) (auth.InviteMemberResult, error) {
	if r.inviteErr != nil {
		return auth.InviteMemberResult{}, r.inviteErr
	}
	res := r.inviteResult
	if res.TenantID == "" {
		res.TenantID = input.TenantID
	}
	return res, nil
}
func (r *memoryRepository) RevokeTenantMember(context.Context, string, string, string) error {
	return r.revokeErr
}

func signingKey(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func (r *memoryRepository) CreateUser(_ context.Context, email, passwordHash string, role auth.Role) (auth.User, error) {
	if r.created {
		return auth.User{}, auth.ErrEmailTaken
	}
	r.created = true
	r.passwordHash = passwordHash
	r.user = auth.User{ID: "user-1", Email: email, Role: role, Scope: "platform"}
	return r.user, nil
}

func (r *memoryRepository) UserByEmail(_ context.Context, _ string) (auth.UserCredential, error) {
	if !r.created {
		return auth.UserCredential{}, errors.New("not found")
	}
	return auth.UserCredential{User: r.user, PasswordHash: r.passwordHash}, nil
}

func (r *memoryRepository) UserByID(_ context.Context, id string) (auth.User, error) {
	if !r.created || r.user.ID != id {
		return auth.User{}, auth.ErrUserNotFound
	}
	return r.user, nil
}

func (r *memoryRepository) BootstrapAdmin(_ context.Context, email, passwordHash string) error {
	if r.created {
		if r.user.Role == auth.RoleAdmin {
			return nil
		}
		return auth.ErrEmailTaken
	}
	r.created = true
	r.passwordHash = passwordHash
	r.user = auth.User{ID: "admin-1", Email: email, Role: auth.RoleAdmin, Scope: "platform"}
	return nil
}

func (r *memoryRepository) HasAdmin(context.Context) (bool, error) {
	return r.created && r.user.Role == auth.RoleAdmin, nil
}

func (r *memoryRepository) ProvisionOwner(_ context.Context, input auth.ProvisionOwnerInput) (auth.ProvisionOwnerResult, error) {
	r.provisionInput = input
	r.provisionCalls++
	return r.provisionResult, r.provisionErr
}

func (r *memoryRepository) Ping(context.Context) error { return nil }
func (r *memoryRepository) Close()                     {}

func TestRegistrationAlwaysCreatesUserAndHashesPassword(t *testing.T) {
	repo := &memoryRepository{}
	privateKey := signingKey(t)
	service := auth.NewService(repo, privateKey, time.Minute)

	session, err := service.Register(context.Background(), " User@Example.com ", "a long password 123")
	if err != nil {
		t.Fatal(err)
	}
	if session.User.Role != auth.RoleUser || session.User.Email != "user@example.com" {
		t.Fatalf("unexpected user: %+v", session.User)
	}
	if repo.passwordHash == "a long password 123" || bcrypt.CompareHashAndPassword([]byte(repo.passwordHash), []byte("a long password 123")) != nil {
		t.Fatal("password was not stored as a valid bcrypt hash")
	}
	claims, err := auth.VerifyToken(privateKey.Public().(ed25519.PublicKey), session.AccessToken, time.Now(), auth.AudienceAPI)
	if err != nil || claims.Role != auth.RoleUser {
		t.Fatalf("invalid registration token claims: %+v, %v", claims, err)
	}
}

func TestRegistrationRejectsShortPassword(t *testing.T) {
	repo := &memoryRepository{}
	service := auth.NewService(repo, signingKey(t), time.Minute)
	_, err := service.Register(context.Background(), "user@example.com", "short")
	if !errors.Is(err, auth.ErrInvalidInput) || repo.created {
		t.Fatalf("err = %v, created = %v", err, repo.created)
	}
}

func TestBootstrapCreatesAdminRole(t *testing.T) {
	repo := &memoryRepository{}
	service := auth.NewService(repo, signingKey(t), time.Minute)
	if err := service.BootstrapAdmin(context.Background(), "admin@example.com", "a very strong password"); err != nil {
		t.Fatal(err)
	}
	if repo.user.Role != auth.RoleAdmin {
		t.Fatalf("bootstrap role = %q, want admin", repo.user.Role)
	}
	if bcrypt.CompareHashAndPassword([]byte(repo.passwordHash), []byte("a very strong password")) != nil {
		t.Fatal("bootstrap admin password is not stored as a valid hash")
	}
}

func TestBootstrapDoesNotResetExistingAdmin(t *testing.T) {
	repo := &memoryRepository{}
	service := auth.NewService(repo, signingKey(t), time.Minute)
	if err := service.BootstrapAdmin(context.Background(), "admin@example.com", "a very strong password"); err != nil {
		t.Fatal(err)
	}
	originalHash := repo.passwordHash
	if err := service.BootstrapAdmin(context.Background(), "other@example.com", "another strong password"); err != nil {
		t.Fatal(err)
	}
	if err := service.BootstrapAdmin(context.Background(), "", ""); err != nil {
		t.Fatal(err)
	}
	if repo.user.Email != "admin@example.com" || repo.passwordHash != originalHash {
		t.Fatal("repeated bootstrap changed the existing administrator")
	}
}

func TestBootstrapRequiresCredentialsOnFreshDatabase(t *testing.T) {
	repo := &memoryRepository{}
	service := auth.NewService(repo, signingKey(t), time.Minute)
	if err := service.BootstrapAdmin(context.Background(), "", ""); err == nil || repo.created {
		t.Fatalf("bootstrap without credentials: err = %v, created = %v", err, repo.created)
	}
}

func TestBootstrapCannotElevateExistingUser(t *testing.T) {
	repo := &memoryRepository{created: true, user: auth.User{Email: "user@example.com", Role: auth.RoleUser}, passwordHash: "original hash"}
	service := auth.NewService(repo, signingKey(t), time.Minute)
	err := service.BootstrapAdmin(context.Background(), "user@example.com", "a very strong password")
	if !errors.Is(err, auth.ErrEmailTaken) || repo.user.Role != auth.RoleUser || repo.passwordHash != "original hash" {
		t.Fatalf("bootstrap elevated existing user: err = %v, user = %+v", err, repo.user)
	}
}

func TestExchangeTenantSessionRequiresActiveMembership(t *testing.T) {
	tenantID := "11111111-1111-1111-1111-111111111111"
	otherTenantID := "22222222-2222-2222-2222-222222222222"
	repo := &memoryRepository{
		created: true,
		user:    auth.User{ID: "user-1", Email: "owner@example.com", Role: auth.RoleUser, Scope: "tenant"},
		activeMembership: auth.TenantMembership{
			MembershipID:      "mem-1",
			UserID:            "user-1",
			TenantID:          tenantID,
			RoleCode:          "owner",
			PermissionProfile: "tenant_owner",
			Status:            "ACTIVE",
		},
	}
	privateKey := signingKey(t)
	service := auth.NewService(repo, privateKey, time.Minute)
	baseToken, _, err := auth.SignToken(privateKey, "user-1", auth.RoleUser, "tenant", time.Now().UTC(), time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	session, err := service.ExchangeTenantSession(context.Background(), baseToken, tenantID)
	if err != nil {
		t.Fatalf("ExchangeTenantSession failed: %v", err)
	}
	if session.TenantID != tenantID || session.PermissionProfile != "tenant_owner" {
		t.Fatalf("unexpected tenant session: %+v", session)
	}
	claims, err := auth.VerifyToken(privateKey.Public().(ed25519.PublicKey), session.AccessToken, time.Now(), auth.AudienceAPI)
	if err != nil {
		t.Fatalf("VerifyToken failed: %v", err)
	}
	if claims.Scope != "tenant" || claims.TenantID != tenantID || claims.PermissionProfile != "tenant_owner" {
		t.Fatalf("unexpected tenant token claims: %+v", claims)
	}

	if _, err := service.ExchangeTenantSession(context.Background(), baseToken, otherTenantID); !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("expected ErrForbidden for non-member tenant, got %v", err)
	}
}

func TestInviteAndRevokeTenantMemberEnforcesOwnerRole(t *testing.T) {
	tenantID := "11111111-1111-1111-1111-111111111111"
	membershipID := "33333333-3333-3333-3333-333333333333"
	privateKey := signingKey(t)
	token, _, err := auth.SignTenantToken(privateKey, "user-1", tenantID, "tenant_owner", time.Now().UTC(), time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	repo := &memoryRepository{
		created: true,
		user:    auth.User{ID: "user-1", Email: "member@example.com", Role: auth.RoleUser, Scope: "tenant"},
		activeMembership: auth.TenantMembership{
			MembershipID:      "mem-viewer",
			UserID:            "user-1",
			TenantID:          tenantID,
			RoleCode:          "viewer",
			PermissionProfile: "tenant_viewer",
			Status:            "ACTIVE",
		},
		inviteResult: auth.InviteMemberResult{
			TenantID:     tenantID,
			UserID:       "user-2",
			MembershipID: membershipID,
			InvitationID: "inv-2",
			Status:       "PENDING",
		},
	}
	service := auth.NewService(repo, privateKey, time.Minute)

	if _, err := service.InviteTenantMember(context.Background(), token, tenantID, auth.InviteMemberInput{
		Email:             "staff@example.com",
		PermissionProfile: "tenant_viewer",
	}); !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("expected viewer invite to fail with ErrForbidden, got %v", err)
	}

	repo.activeMembership.RoleCode = "owner"
	repo.activeMembership.PermissionProfile = "tenant_owner"

	res, err := service.InviteTenantMember(context.Background(), token, tenantID, auth.InviteMemberInput{
		Email:             "staff@example.com",
		PermissionProfile: "tenant_viewer",
	})
	if err != nil || res.MembershipID != membershipID {
		t.Fatalf("InviteTenantMember failed: res=%+v, err=%v", res, err)
	}

	if err := service.RevokeTenantMember(context.Background(), token, tenantID, membershipID); err != nil {
		t.Fatalf("RevokeTenantMember failed: %v", err)
	}
}
