package tests

import (
	"context"
	"errors"
	"testing"
	"time"

	"darksamus.com/ReserveHub/auth-service/internal/auth"
	"golang.org/x/crypto/bcrypt"
)

type memoryRepository struct {
	user         auth.User
	passwordHash string
	created      bool
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
		return auth.User{}, errors.New("not found")
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

func (r *memoryRepository) Ping(context.Context) error { return nil }
func (r *memoryRepository) Close()                     {}

func TestRegistrationAlwaysCreatesUserAndHashesPassword(t *testing.T) {
	repo := &memoryRepository{}
	secret := []byte("0123456789abcdef0123456789abcdef")
	service := auth.NewService(repo, secret, time.Minute)

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
	claims, err := auth.VerifyToken(secret, session.AccessToken, time.Now())
	if err != nil || claims.Role != auth.RoleUser {
		t.Fatalf("invalid registration token claims: %+v, %v", claims, err)
	}
}

func TestRegistrationRejectsShortPassword(t *testing.T) {
	repo := &memoryRepository{}
	service := auth.NewService(repo, []byte("0123456789abcdef0123456789abcdef"), time.Minute)
	_, err := service.Register(context.Background(), "user@example.com", "short")
	if !errors.Is(err, auth.ErrInvalidInput) || repo.created {
		t.Fatalf("err = %v, created = %v", err, repo.created)
	}
}

func TestBootstrapCreatesAdminRole(t *testing.T) {
	repo := &memoryRepository{}
	service := auth.NewService(repo, []byte("0123456789abcdef0123456789abcdef"), time.Minute)
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
	service := auth.NewService(repo, []byte("0123456789abcdef0123456789abcdef"), time.Minute)
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
	service := auth.NewService(repo, []byte("0123456789abcdef0123456789abcdef"), time.Minute)
	if err := service.BootstrapAdmin(context.Background(), "", ""); err == nil || repo.created {
		t.Fatalf("bootstrap without credentials: err = %v, created = %v", err, repo.created)
	}
}

func TestBootstrapCannotElevateExistingUser(t *testing.T) {
	repo := &memoryRepository{created: true, user: auth.User{Email: "user@example.com", Role: auth.RoleUser}, passwordHash: "original hash"}
	service := auth.NewService(repo, []byte("0123456789abcdef0123456789abcdef"), time.Minute)
	err := service.BootstrapAdmin(context.Background(), "user@example.com", "a very strong password")
	if !errors.Is(err, auth.ErrEmailTaken) || repo.user.Role != auth.RoleUser || repo.passwordHash != "original hash" {
		t.Fatalf("bootstrap elevated existing user: err = %v, user = %+v", err, repo.user)
	}
}
