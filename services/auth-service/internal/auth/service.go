package auth

import (
	"context"
	"crypto/ed25519"
	"time"

	"github.com/google/uuid"
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
	repo       Repository
	privateKey ed25519.PrivateKey
	ttl        time.Duration
	now        func() time.Time
}

func NewService(repo Repository, privateKey ed25519.PrivateKey, ttl time.Duration) *Service {
	return &Service{repo: repo, privateKey: append(ed25519.PrivateKey(nil), privateKey...), ttl: ttl, now: time.Now}
}

func (s *Service) Ready(ctx context.Context) error { return s.repo.Ping(ctx) }

func (s *Service) newSession(user User) (Session, error) {
	now := s.now().UTC()
	token, expiresAt, err := SignToken(s.privateKey, user.ID, user.Role, user.Scope, now, s.ttl)
	if err != nil {
		return Session{}, err
	}
	return Session{AccessToken: token, TokenType: "Bearer", ExpiresAt: expiresAt, User: user}, nil
}

func NewUserID() string { return uuid.NewString() }
