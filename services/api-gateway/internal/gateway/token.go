package gateway

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const (
	tokenIssuer   = "reservehub-auth"
	tokenAudience = "reservehub-api"
	tokenKeyID    = "auth-ed25519-v1"
)

type tokenClaims struct {
	Subject  string   `json:"sub"`
	Role     string   `json:"role"`
	Scope    string   `json:"scope"`
	Audience []string `json:"aud"`
	Issuer   string   `json:"iss"`
	IssuedAt int64    `json:"iat"`
	Expires  int64    `json:"exp"`
}

var errInvalidToken = errors.New("invalid bearer token")

func verifyToken(publicKey ed25519.PublicKey, token string, now time.Time) (tokenClaims, error) {
	var claims tokenClaims
	if len(publicKey) != ed25519.PublicKeySize || len(token) > 8192 {
		return claims, errInvalidToken
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return claims, errInvalidToken
	}
	decode := base64.RawURLEncoding.DecodeString
	headerJSON, err := decode(parts[0])
	if err != nil {
		return claims, errInvalidToken
	}
	var header struct {
		Algorithm string `json:"alg"`
		Type      string `json:"typ"`
		KeyID     string `json:"kid"`
	}
	if json.Unmarshal(headerJSON, &header) != nil || header.Algorithm != "EdDSA" || header.Type != "JWT" || header.KeyID != tokenKeyID {
		return claims, errInvalidToken
	}
	signature, err := decode(parts[2])
	if err != nil || !ed25519.Verify(publicKey, []byte(parts[0]+"."+parts[1]), signature) {
		return claims, errInvalidToken
	}
	payload, err := decode(parts[1])
	if err != nil || json.Unmarshal(payload, &claims) != nil {
		return tokenClaims{}, errInvalidToken
	}
	current := now.Unix()
	if claims.Subject == "" || claims.Issuer != tokenIssuer || (claims.Role != "admin" && claims.Role != "user") ||
		(claims.Scope != "platform" && claims.Scope != "tenant") || !hasAudience(claims.Audience, tokenAudience) ||
		claims.Expires <= current || claims.IssuedAt <= 0 || claims.IssuedAt > now.Add(time.Minute).Unix() ||
		claims.Expires <= claims.IssuedAt || claims.Expires-claims.IssuedAt > int64(15*time.Minute/time.Second) {
		return tokenClaims{}, errInvalidToken
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
