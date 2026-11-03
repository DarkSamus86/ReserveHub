package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	Issuer         = "reservehub-auth"
	AudienceAPI    = "reservehub-api"
	AudienceTenant = "reservehub-tenant"
	TokenKeyID     = "auth-ed25519-v1"
)

type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"

	ProfileTenantOwner  = "tenant_owner"
	ProfileTenantViewer = "tenant_viewer"
)

type Claims struct {
	Subject           string   `json:"sub"`
	Role              Role     `json:"role"`
	Scope             string   `json:"scope"`
	TenantID          string   `json:"tenant_id,omitempty"`
	PermissionProfile string   `json:"permission_profile,omitempty"`
	Audience          []string `json:"aud"`
	Issuer            string   `json:"iss"`
	IssuedAt          int64    `json:"iat"`
	Expires           int64    `json:"exp"`
}

var ErrInvalidToken = errors.New("invalid access token")

func SignToken(privateKey ed25519.PrivateKey, userID string, role Role, scope string, now time.Time, ttl time.Duration) (string, time.Time, error) {
	if len(privateKey) != ed25519.PrivateKeySize || userID == "" || (role != RoleAdmin && role != RoleUser) || (scope != "platform" && scope != "tenant") || ttl <= 0 || ttl > 15*time.Minute {
		return "", time.Time{}, errors.New("invalid token signing configuration")
	}
	expiresAt := now.Add(ttl).UTC()
	claims := Claims{
		Subject: userID, Role: role, Scope: scope,
		Audience: []string{AudienceAPI, AudienceTenant}, Issuer: Issuer,
		IssuedAt: now.Unix(), Expires: expiresAt.Unix(),
	}
	return signClaims(privateKey, claims, expiresAt)
}

func SignTenantToken(privateKey ed25519.PrivateKey, userID, tenantID, permissionProfile string, now time.Time, ttl time.Duration) (string, time.Time, error) {
	parsedTenant, err := uuid.Parse(tenantID)
	if len(privateKey) != ed25519.PrivateKeySize || userID == "" || err != nil || parsedTenant.String() != tenantID ||
		(permissionProfile != ProfileTenantOwner && permissionProfile != ProfileTenantViewer) || ttl <= 0 || ttl > 15*time.Minute {
		return "", time.Time{}, errors.New("invalid tenant token signing configuration")
	}
	expiresAt := now.Add(ttl).UTC()
	claims := Claims{
		Subject:           userID,
		Role:              RoleUser,
		Scope:             "tenant",
		TenantID:          tenantID,
		PermissionProfile: permissionProfile,
		Audience:          []string{AudienceAPI, AudienceTenant},
		Issuer:            Issuer,
		IssuedAt:          now.Unix(),
		Expires:           expiresAt.Unix(),
	}
	return signClaims(privateKey, claims, expiresAt)
}

func signClaims(privateKey ed25519.PrivateKey, claims Claims, expiresAt time.Time) (string, time.Time, error) {
	header, err := json.Marshal(map[string]string{"alg": "EdDSA", "typ": "JWT", "kid": TokenKeyID})
	if err != nil {
		return "", time.Time{}, err
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", time.Time{}, err
	}
	encode := base64.RawURLEncoding.EncodeToString
	unsigned := encode(header) + "." + encode(payload)
	signature := ed25519.Sign(privateKey, []byte(unsigned))
	return unsigned + "." + encode(signature), expiresAt, nil
}

func VerifyToken(publicKey ed25519.PublicKey, token string, now time.Time, audience string) (Claims, error) {
	var claims Claims
	if len(publicKey) != ed25519.PublicKeySize || len(token) > 8192 || audience == "" {
		return claims, ErrInvalidToken
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return claims, ErrInvalidToken
	}
	decode := base64.RawURLEncoding.DecodeString
	headerJSON, err := decode(parts[0])
	if err != nil {
		return claims, ErrInvalidToken
	}
	var header struct {
		Algorithm string `json:"alg"`
		Type      string `json:"typ"`
		KeyID     string `json:"kid"`
	}
	if json.Unmarshal(headerJSON, &header) != nil || header.Algorithm != "EdDSA" || header.Type != "JWT" || header.KeyID != TokenKeyID {
		return claims, ErrInvalidToken
	}
	signature, err := decode(parts[2])
	if err != nil || !ed25519.Verify(publicKey, []byte(parts[0]+"."+parts[1]), signature) {
		return claims, ErrInvalidToken
	}
	payload, err := decode(parts[1])
	if err != nil || json.Unmarshal(payload, &claims) != nil {
		return Claims{}, ErrInvalidToken
	}
	current := now.Unix()
	if claims.Subject == "" || claims.Issuer != Issuer || (claims.Role != RoleAdmin && claims.Role != RoleUser) ||
		(claims.Scope != "platform" && claims.Scope != "tenant") || !hasAudience(claims.Audience, audience) ||
		claims.Expires <= current || claims.IssuedAt <= 0 || claims.IssuedAt > now.Add(time.Minute).Unix() ||
		claims.Expires <= claims.IssuedAt || claims.Expires-claims.IssuedAt > int64(15*time.Minute/time.Second) {
		return Claims{}, ErrInvalidToken
	}
	if claims.Scope == "tenant" && claims.TenantID != "" {
		parsedTenant, err := uuid.Parse(claims.TenantID)
		if err != nil || parsedTenant.String() != claims.TenantID ||
			(claims.PermissionProfile != ProfileTenantOwner && claims.PermissionProfile != ProfileTenantViewer) {
			return Claims{}, ErrInvalidToken
		}
	}
	return claims, nil
}

func hasAudience(audiences []string, expected string) bool {
	for _, audience := range audiences {
		if audience == expected {
			return true
		}
	}
	return false
}

func GenerateKeyPair() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	return ed25519.GenerateKey(rand.Reader)
}
