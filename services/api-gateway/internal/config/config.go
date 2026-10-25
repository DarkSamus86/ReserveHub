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
	HTTPAddress    string
	AuthServiceURL *url.URL
	JWTVerifyKey   ed25519.PublicKey
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
	return Config{HTTPAddress: address, AuthServiceURL: upstream, JWTVerifyKey: append(ed25519.PublicKey(nil), key...)}, nil
}

func env(name, fallback string) string {
	if value, ok := os.LookupEnv(name); ok {
		return value
	}
	return fallback
}
