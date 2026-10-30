package auth

import "errors"

var (
	ErrInvalidInput       = errors.New("invalid input")
	ErrEmailTaken         = errors.New("email already registered")
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUnauthenticated    = errors.New("unauthenticated")
	ErrUserNotFound       = errors.New("user not found")
	ErrProvisionConflict  = errors.New("provision request conflicts with an existing operation")
	ErrInvalidSecret      = errors.New("invalid or expired secret")
	ErrInvitationNotFound = errors.New("invitation not found")
)
