package authn

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestVerifyPlatformAdminRequiresPlatformRole(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	sign := func(role, scope string) string {
		header, _ := json.Marshal(map[string]string{"alg": "EdDSA", "typ": "JWT", "kid": "auth-ed25519-v1"})
		claims, _ := json.Marshal(map[string]any{"sub": uuid.NewString(), "role": role, "scope": scope, "aud": []string{"reservehub-api", "reservehub-tenant"}, "iss": "reservehub-auth", "iat": now.Unix(), "exp": now.Add(time.Minute).Unix()})
		encode := base64.RawURLEncoding.EncodeToString
		unsigned := encode(header) + "." + encode(claims)
		return unsigned + "." + encode(ed25519.Sign(privateKey, []byte(unsigned)))
	}
	if _, ok := VerifyPlatformAdmin(publicKey, sign("admin", "platform"), now); !ok {
		t.Fatal("platform admin rejected")
	}
	if _, ok := VerifyPlatformAdmin(publicKey, sign("admin", "tenant"), now); ok {
		t.Fatal("tenant admin accepted")
	}
	if _, ok := VerifyPlatformAdmin(publicKey, sign("user", "platform"), now); ok {
		t.Fatal("platform user accepted")
	}
}
