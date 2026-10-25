package authn

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const (
	issuer          = "reservehub-auth"
	tenantAudience  = "reservehub-tenant"
	verificationKey = "auth-ed25519-v1"
)

type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

type Claims struct {
	Subject  string   `json:"sub"`
	Role     Role     `json:"role"`
	Scope    string   `json:"scope"`
	Audience []string `json:"aud"`
	Issuer   string   `json:"iss"`
	IssuedAt int64    `json:"iat"`
	Expires  int64    `json:"exp"`
}

var ErrInvalidToken = errors.New("invalid access token")

func VerifyToken(publicKey ed25519.PublicKey, token string, now time.Time) (Claims, error) {
	var claims Claims
	if len(publicKey) != ed25519.PublicKeySize || len(token) > 8192 {
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
	if json.Unmarshal(headerJSON, &header) != nil || header.Algorithm != "EdDSA" || header.Type != "JWT" || header.KeyID != verificationKey {
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
	if claims.Subject == "" || claims.Issuer != issuer || (claims.Role != RoleAdmin && claims.Role != RoleUser) ||
		(claims.Scope != "platform" && claims.Scope != "tenant") || !hasAudience(claims.Audience, tenantAudience) ||
		claims.Expires <= current || claims.IssuedAt <= 0 || claims.IssuedAt > now.Add(time.Minute).Unix() ||
		claims.Expires <= claims.IssuedAt || claims.Expires-claims.IssuedAt > int64(15*time.Minute/time.Second) {
		return Claims{}, ErrInvalidToken
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
