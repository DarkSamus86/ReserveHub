package config

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"time"
)

type Config struct {
	HTTPAddress            string
	DatabaseURL            string
	JWTPrivateKey          ed25519.PrivateKey
	TokenTTL               time.Duration
	BootstrapAdminEmail    string
	BootstrapAdminPassword string
}

func Load(requireJWTKey bool) (Config, error) {
	cfg := Config{
		HTTPAddress:            env("HTTP_ADDRESS", ":8082"),
		DatabaseURL:            env("DATABASE_URL", "postgres://reservehub:reservehub@localhost:5432/reservehub?sslmode=disable&search_path=auth_service"),
		BootstrapAdminEmail:    os.Getenv("AUTH_BOOTSTRAP_ADMIN_EMAIL"),
		BootstrapAdminPassword: os.Getenv("AUTH_BOOTSTRAP_ADMIN_PASSWORD"),
		TokenTTL:               15 * time.Minute,
	}
	if requireJWTKey {
		path := os.Getenv("AUTH_JWT_PRIVATE_KEY_FILE")
		if path == "" {
			return Config{}, fmt.Errorf("AUTH_JWT_PRIVATE_KEY_FILE is required")
		}
		encoded, err := os.ReadFile(path)
		if err != nil {
			return Config{}, fmt.Errorf("read Auth JWT private key: %w", err)
		}
		block, _ := pem.Decode(encoded)
		if block == nil {
			return Config{}, fmt.Errorf("AUTH_JWT_PRIVATE_KEY_FILE is not valid PEM")
		}
		parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return Config{}, fmt.Errorf("parse Auth JWT private key: %w", err)
		}
		key, ok := parsed.(ed25519.PrivateKey)
		if !ok || len(key) != ed25519.PrivateKeySize {
			return Config{}, fmt.Errorf("Auth JWT private key must be Ed25519 PKCS#8")
		}
		cfg.JWTPrivateKey = append(ed25519.PrivateKey(nil), key...)
	}
	if (cfg.BootstrapAdminEmail == "") != (cfg.BootstrapAdminPassword == "") {
		return Config{}, fmt.Errorf("AUTH_BOOTSTRAP_ADMIN_EMAIL and AUTH_BOOTSTRAP_ADMIN_PASSWORD must be set together")
	}
	return cfg, nil
}

func env(name, fallback string) string {
	if value, ok := os.LookupEnv(name); ok {
		return value
	}
	return fallback
}
