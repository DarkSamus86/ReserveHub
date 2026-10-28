package authn

import (
	"crypto/ed25519"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

func PlatformAdminFromRequest(r *http.Request, key ed25519.PublicKey, now time.Time) (uuid.UUID, string, bool) {
	values := r.Header.Values("Authorization")
	if len(values) != 1 {
		return uuid.Nil, "", false
	}
	parts := strings.Fields(values[0])
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return uuid.Nil, "", false
	}
	actor, ok := VerifyPlatformAdmin(key, parts[1], now)
	return actor, parts[1], ok
}

func VerifyPlatformAdmin(key ed25519.PublicKey, raw string, now time.Time) (uuid.UUID, bool) {
	if len(raw) > 8192 {
		return uuid.Nil, false
	}
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return uuid.Nil, false
	}
	decode := base64.RawURLEncoding.DecodeString
	headerBytes, err := decode(parts[0])
	if err != nil {
		return uuid.Nil, false
	}
	var header struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
		Kid string `json:"kid"`
	}
	if json.Unmarshal(headerBytes, &header) != nil || header.Alg != "EdDSA" || header.Typ != "JWT" || header.Kid != "auth-ed25519-v1" {
		return uuid.Nil, false
	}
	signature, err := decode(parts[2])
	if err != nil || !ed25519.Verify(key, []byte(parts[0]+"."+parts[1]), signature) {
		return uuid.Nil, false
	}
	payload, err := decode(parts[1])
	if err != nil {
		return uuid.Nil, false
	}
	var claims struct {
		Sub   string   `json:"sub"`
		Role  string   `json:"role"`
		Scope string   `json:"scope"`
		Aud   []string `json:"aud"`
		Iss   string   `json:"iss"`
		Iat   int64    `json:"iat"`
		Exp   int64    `json:"exp"`
	}
	if json.Unmarshal(payload, &claims) != nil {
		return uuid.Nil, false
	}
	actor, err := uuid.Parse(claims.Sub)
	if err != nil || claims.Role != "admin" || claims.Scope != "platform" || claims.Iss != "reservehub-auth" || claims.Iat <= 0 || claims.Iat > now.Add(time.Minute).Unix() || claims.Exp <= now.Unix() || claims.Exp-claims.Iat > 900 {
		return uuid.Nil, false
	}
	api, tenant := false, false
	for _, aud := range claims.Aud {
		if aud == "reservehub-api" {
			api = true
		}
		if aud == "reservehub-tenant" {
			tenant = true
		}
	}
	if !api || !tenant || subtle.ConstantTimeCompare([]byte(claims.Role), []byte("admin")) != 1 {
		return uuid.Nil, false
	}
	return actor, true
}
