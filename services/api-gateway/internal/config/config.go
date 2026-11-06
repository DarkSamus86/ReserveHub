package config

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	HTTPAddress         string
	AuthServiceURL      *url.URL
	AdminServiceURL     *url.URL
	WorkspaceServiceURL *url.URL
	JWTVerifyKey        ed25519.PublicKey
	AppEnv              string
	// SwaggerEnabled exposes /swagger and /openapi.json. It is false in production.
	SwaggerEnabled bool
}

func Load() (Config, error) {
	address := env("HTTP_ADDRESS", ":8080")
	if strings.TrimSpace(address) == "" {
		return Config{}, fmt.Errorf("HTTP_ADDRESS must not be empty")
	}

	upstream, err := url.Parse(env("AUTH_SERVICE_URL", "http://localhost:8082"))
	if err != nil || (upstream.Scheme != "http" && upstream.Scheme != "https") || upstream.Host == "" || upstream.User != nil || upstream.RawQuery != "" || upstream.Fragment != "" || (upstream.Path != "" && upstream.Path != "/") {
		return Config{}, fmt.Errorf("AUTH_SERVICE_URL must be an absolute http(s) URL without credentials, query, fragment, or path")
	}
	upstream.Path = ""
	adminUpstream, err := url.Parse(env("ADMIN_SERVICE_URL", "http://localhost:8083"))
	if err != nil || (adminUpstream.Scheme != "http" && adminUpstream.Scheme != "https") || adminUpstream.Host == "" || adminUpstream.User != nil || adminUpstream.RawQuery != "" || adminUpstream.Fragment != "" || (adminUpstream.Path != "" && adminUpstream.Path != "/") {
		return Config{}, fmt.Errorf("ADMIN_SERVICE_URL must be an absolute http(s) URL without credentials, query, fragment, or path")
	}
	adminUpstream.Path = ""
	workspaceUpstream, err := url.Parse(env("WORKSPACE_SERVICE_URL", "http://localhost:8085"))
	if err != nil || (workspaceUpstream.Scheme != "http" && workspaceUpstream.Scheme != "https") || workspaceUpstream.Host == "" || workspaceUpstream.User != nil || workspaceUpstream.RawQuery != "" || workspaceUpstream.Fragment != "" || (workspaceUpstream.Path != "" && workspaceUpstream.Path != "/") {
		return Config{}, fmt.Errorf("WORKSPACE_SERVICE_URL must be an absolute http(s) URL without credentials, query, fragment, or path")
	}
	workspaceUpstream.Path = ""

	appEnv := strings.ToLower(strings.TrimSpace(env("APP_ENV", "production")))
	if appEnv == "" {
		appEnv = "production"
	}
	// Fail closed: Swagger is exposed only in explicitly non-production environments.
	swaggerEnabled := false
	switch appEnv {
	case "local", "dev", "development", "test", "testing", "staging":
		swaggerEnabled = true
	case "prod", "production":
	default:
		return Config{}, fmt.Errorf("APP_ENV must be one of local, dev, test, staging, prod, production")
	}

	keyPath := os.Getenv("AUTH_JWT_PUBLIC_KEY_FILE")
	if keyPath == "" {
		return Config{}, fmt.Errorf("AUTH_JWT_PUBLIC_KEY_FILE is required")
	}
	encoded, err := os.ReadFile(keyPath)
	if err != nil {
		return Config{}, fmt.Errorf("read Auth JWT public key: %w", err)
	}
	block, _ := pem.Decode(encoded)
	if block == nil {
		return Config{}, fmt.Errorf("AUTH_JWT_PUBLIC_KEY_FILE is not valid PEM")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return Config{}, fmt.Errorf("parse Auth JWT public key: %w", err)
	}
	key, ok := parsed.(ed25519.PublicKey)
	if !ok || len(key) != ed25519.PublicKeySize {
		return Config{}, fmt.Errorf("Auth JWT public key must be Ed25519 PKIX")
	}
	return Config{HTTPAddress: address, AuthServiceURL: upstream, AdminServiceURL: adminUpstream, WorkspaceServiceURL: workspaceUpstream, JWTVerifyKey: append(ed25519.PublicKey(nil), key...), AppEnv: appEnv, SwaggerEnabled: swaggerEnabled}, nil
}

func env(name, fallback string) string {
	if value, ok := os.LookupEnv(name); ok {
		return value
	}
	return fallback
}
