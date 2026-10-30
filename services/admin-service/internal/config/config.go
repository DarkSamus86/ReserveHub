package config

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
)

type Config struct {
	HTTPAddress    string
	DatabaseURL    string
	TenantAddress  string
	AuthServiceURL string
	PublicKey      ed25519.PublicKey
	ServiceToken   string
	NATSURL        string
	OutboxInterval time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddress:    env("HTTP_ADDRESS", ":8083"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		TenantAddress:  os.Getenv("TENANT_GRPC_ADDRESS"),
		AuthServiceURL: os.Getenv("AUTH_SERVICE_URL"),
		NATSURL:        os.Getenv("NATS_URL"),
		OutboxInterval: time.Second,
	}
	if cfg.DatabaseURL == "" || cfg.TenantAddress == "" || cfg.AuthServiceURL == "" || cfg.NATSURL == "" {
		return Config{}, errors.New("DATABASE_URL, TENANT_GRPC_ADDRESS, AUTH_SERVICE_URL and NATS_URL are required")
	}
	if value := os.Getenv("OUTBOX_INTERVAL"); value != "" {
		interval, err := time.ParseDuration(value)
		if err != nil || interval <= 0 {
			return Config{}, errors.New("OUTBOX_INTERVAL must be a positive duration")
		}
		cfg.OutboxInterval = interval
	}
	authURL, err := url.Parse(cfg.AuthServiceURL)
	if err != nil || (authURL.Scheme != "http" && authURL.Scheme != "https") || authURL.Host == "" || authURL.User != nil || authURL.RawQuery != "" || authURL.Fragment != "" || (authURL.Path != "" && authURL.Path != "/") {
		return Config{}, errors.New("AUTH_SERVICE_URL must be an absolute http(s) URL without credentials, query, fragment, or path")
	}
	keyFile := os.Getenv("AUTH_JWT_PUBLIC_KEY_FILE")
	if keyFile == "" {
		return Config{}, errors.New("AUTH_JWT_PUBLIC_KEY_FILE is required")
	}
	encoded, err := os.ReadFile(keyFile)
	if err != nil {
		return Config{}, fmt.Errorf("read public key: %w", err)
	}
	block, _ := pem.Decode(encoded)
	if block == nil {
		return Config{}, errors.New("invalid public key PEM")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return Config{}, fmt.Errorf("parse public key: %w", err)
	}
	key, ok := parsed.(ed25519.PublicKey)
	if !ok || len(key) != ed25519.PublicKeySize {
		return Config{}, errors.New("public key must be Ed25519")
	}
	cfg.PublicKey = key
	tokenFile := os.Getenv("PLATFORM_SERVICE_TOKEN_FILE")
	if tokenFile == "" {
		return Config{}, errors.New("PLATFORM_SERVICE_TOKEN_FILE is required")
	}
	encoded, err = os.ReadFile(tokenFile)
	if err != nil {
		return Config{}, fmt.Errorf("read platform service token: %w", err)
	}
	cfg.ServiceToken = strings.TrimSpace(string(encoded))
	if len(cfg.ServiceToken) < 32 {
		return Config{}, errors.New("platform service token must be at least 32 bytes")
	}
	return cfg, nil
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
