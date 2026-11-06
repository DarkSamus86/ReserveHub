package authn

import (
	"crypto/ed25519"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	ProfileTenantOwner  = "tenant_owner"
	ProfileTenantViewer = "tenant_viewer"
)

type TenantMemberPrincipal struct {
	UserID            uuid.UUID
	TenantID          uuid.UUID
	PermissionProfile string
}

func VerifyPlatformAdmin(key ed25519.PublicKey, raw string, now time.Time) (uuid.UUID, bool) {
	claims, ok := parseAndVerifyClaims(key, raw, now)
	if !ok {
		return uuid.Nil, false
	}
	actor, err := uuid.Parse(claims.Sub)
	if err != nil || claims.Scope != "platform" || subtle.ConstantTimeCompare([]byte(claims.Role), []byte("admin")) != 1 {
		return uuid.Nil, false
	}
	return actor, true
}

func VerifyTenantMember(key ed25519.PublicKey, raw string, now time.Time) (TenantMemberPrincipal, bool) {
	claims, ok := parseAndVerifyClaims(key, raw, now)
	if !ok {
		return TenantMemberPrincipal{}, false
	}
	actor, err := uuid.Parse(claims.Sub)
	if err != nil || claims.Scope != "tenant" || subtle.ConstantTimeCompare([]byte(claims.Role), []byte("user")) != 1 {
		return TenantMemberPrincipal{}, false
	}
	tenantID, err := uuid.Parse(claims.TenantID)
	if err != nil || tenantID == uuid.Nil || tenantID.String() != claims.TenantID {
		return TenantMemberPrincipal{}, false
	}
	if claims.PermissionProfile != ProfileTenantOwner && claims.PermissionProfile != ProfileTenantViewer {
		return TenantMemberPrincipal{}, false
	}
	return TenantMemberPrincipal{
		UserID:            actor,
		TenantID:          tenantID,
		PermissionProfile: claims.PermissionProfile,
	}, true
}

type verifiedClaims struct {
	Sub               string   `json:"sub"`
	Role              string   `json:"role"`
	Scope             string   `json:"scope"`
	TenantID          string   `json:"tenant_id,omitempty"`
	PermissionProfile string   `json:"permission_profile,omitempty"`
	Aud               []string `json:"aud"`
	Iss               string   `json:"iss"`
	Iat               int64    `json:"iat"`
	Exp               int64    `json:"exp"`
}

func parseAndVerifyClaims(key ed25519.PublicKey, raw string, now time.Time) (verifiedClaims, bool) {
	var claims verifiedClaims
	if len(key) != ed25519.PublicKeySize || len(raw) > 8192 {
		return claims, false
	}
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return claims, false
	}
	decode := base64.RawURLEncoding.DecodeString
	headerBytes, err := decode(parts[0])
	if err != nil {
		return claims, false
	}
	var header struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
		Kid string `json:"kid"`
	}
	if json.Unmarshal(headerBytes, &header) != nil || header.Alg != "EdDSA" || header.Typ != "JWT" || header.Kid != "auth-ed25519-v1" {
		return claims, false
	}
	signature, err := decode(parts[2])
	if err != nil || !ed25519.Verify(key, []byte(parts[0]+"."+parts[1]), signature) {
		return claims, false
	}
	payload, err := decode(parts[1])
	if err != nil || json.Unmarshal(payload, &claims) != nil {
		return claims, false
	}
	if claims.Iss != "reservehub-auth" || claims.Iat <= 0 || claims.Iat > now.Add(time.Minute).Unix() || claims.Exp <= now.Unix() || claims.Exp-claims.Iat > 900 {
		return claims, false
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
	if !api || !tenant {
		return claims, false
	}
	return claims, true
}
