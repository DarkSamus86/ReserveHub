package authn

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const Issuer = "reservehub-auth"

type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
)

type Claims struct {
	Subject  string `json:"sub"`
	Role     Role   `json:"role"`
	Scope    string `json:"scope"`
	Issuer   string `json:"iss"`
	IssuedAt int64  `json:"iat"`
	Expires  int64  `json:"exp"`
}

var ErrInvalidToken = errors.New("invalid access token")

func VerifyToken(secret []byte, token string, now time.Time) (Claims, error) {
	var claims Claims
	if len(secret) < 32 {
		return claims, ErrInvalidToken
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return claims, ErrInvalidToken
	}
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return claims, ErrInvalidToken
	}
	var header struct {
		Algorithm string `json:"alg"`
		Type      string `json:"typ"`
	}
	if json.Unmarshal(headerJSON, &header) != nil || header.Algorithm != "HS256" || header.Type != "JWT" {
		return claims, ErrInvalidToken
	}
	provided, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(provided, sign(secret, parts[0]+"."+parts[1])) {
		return claims, ErrInvalidToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || json.Unmarshal(payload, &claims) != nil {
		return Claims{}, ErrInvalidToken
	}
	if claims.Subject == "" || claims.Issuer != Issuer || (claims.Role != RoleAdmin && claims.Role != RoleUser) || (claims.Scope != "platform" && claims.Scope != "tenant") ||
		claims.Expires <= now.Unix() || claims.IssuedAt > now.Add(time.Minute).Unix() || claims.Expires <= claims.IssuedAt {
		return Claims{}, ErrInvalidToken
	}
	return claims, nil
}

func sign(secret []byte, message string) []byte {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(message))
	return mac.Sum(nil)
}
