package auth

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidInput       = errors.New("invalid input")
	ErrEmailTaken         = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUnauthenticated    = errors.New("unauthenticated")
	ErrUserNotFound       = errors.New("user not found")
)

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Role      Role      `json:"role"`
	Scope     string    `json:"-"`
	CreatedAt time.Time `json:"created_at"`
}

type UserCredential struct {
	User
	PasswordHash string
}

type Repository interface {
	CreateUser(context.Context, string, string, Role) (User, error)
	UserByEmail(context.Context, string) (UserCredential, error)
	UserByID(context.Context, string) (User, error)
	BootstrapAdmin(context.Context, string, string) error
	HasAdmin(context.Context) (bool, error)
	Ping(context.Context) error
	Close()
}

type Session struct {
	AccessToken string    `json:"access_token"`
	TokenType   string    `json:"token_type"`
	ExpiresAt   time.Time `json:"expires_at"`
	User        User      `json:"user"`
}

type Service struct {
	repo   Repository
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

func NewService(repo Repository, secret []byte, ttl time.Duration) *Service {
	return &Service{repo: repo, secret: append([]byte(nil), secret...), ttl: ttl, now: time.Now}
}

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
	return s.newSession(user)
}

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
	return s.newSession(user.User)
}

func (s *Service) Me(ctx context.Context, token string) (User, error) {
	claims, err := VerifyToken(s.secret, token, s.now())
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

func (s *Service) Ready(ctx context.Context) error { return s.repo.Ping(ctx) }

func (s *Service) newSession(user User) (Session, error) {
	now := s.now().UTC()
	token, expiresAt, err := SignToken(s.secret, user.ID, user.Role, user.Scope, now, s.ttl)
	if err != nil {
		return Session{}, err
	}
	return Session{AccessToken: token, TokenType: "Bearer", ExpiresAt: expiresAt, User: user}, nil
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

func NewUserID() string { return uuid.NewString() }
