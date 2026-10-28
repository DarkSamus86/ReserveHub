package auth

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
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
	ProvisionOwner(context.Context, ProvisionOwnerInput) (ProvisionOwnerResult, error)
	CreateSession(context.Context, string, string, []byte, time.Time) error
	RotateSession(context.Context, string, []byte, []byte, time.Time) (User, time.Time, error)
	RevokeSession(context.Context, string, []byte) error
	AcceptInvitation(context.Context, []byte, string) (AcceptInvitationResult, error)
	ReissueInvitation(context.Context, string) error
	InvitationStatus(context.Context, string) (InvitationStatus, error)
	Ping(context.Context) error
	Close()
}

type Session struct {
	AccessToken      string    `json:"access_token"`
	RefreshToken     string    `json:"refresh_token"`
	TokenType        string    `json:"token_type"`
	ExpiresAt        time.Time `json:"expires_at"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at"`
	User             User      `json:"user"`
}

type Service struct {
	repo       Repository
	privateKey ed25519.PrivateKey
	ttl        time.Duration
	refreshTTL time.Duration
	now        func() time.Time
}

func NewService(repo Repository, privateKey ed25519.PrivateKey, ttl time.Duration) *Service {
	return &Service{repo: repo, privateKey: append(ed25519.PrivateKey(nil), privateKey...), ttl: ttl, refreshTTL: 30 * 24 * time.Hour, now: time.Now}
}

func (s *Service) SetRefreshTTL(ttl time.Duration) { s.refreshTTL = ttl }

func (s *Service) Ready(ctx context.Context) error { return s.repo.Ping(ctx) }

func (s *Service) newSession(ctx context.Context, user User) (Session, error) {
	now := s.now().UTC()
	token, expiresAt, err := SignToken(s.privateKey, user.ID, user.Role, user.Scope, now, s.ttl)
	if err != nil {
		return Session{}, err
	}
	secret, err := newRefreshSecret()
	if err != nil {
		return Session{}, err
	}
	id := uuid.NewString()
	refreshToken := id + "." + secret
	hash := sha256.Sum256([]byte(refreshToken))
	refreshExpiresAt := now.Add(s.refreshTTL)
	err = s.repo.CreateSession(ctx, id, user.ID, hash[:], refreshExpiresAt)
	if err != nil {
		return Session{}, err
	}
	return Session{AccessToken: token, RefreshToken: refreshToken, TokenType: "Bearer", ExpiresAt: expiresAt, RefreshExpiresAt: refreshExpiresAt, User: user}, nil
}

func NewUserID() string { return uuid.NewString() }
