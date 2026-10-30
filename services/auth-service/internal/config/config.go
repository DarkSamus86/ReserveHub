package config

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	HTTPAddress             string
	DatabaseURL             string
	JWTPrivateKey           ed25519.PrivateKey
	TokenTTL                time.Duration
	BootstrapAdminEmail     string
	BootstrapAdminPassword  string
	ProvisionServiceToken   string
	RefreshTTL              time.Duration
	InvitationTTL           time.Duration
	InvitationEncryptionKey []byte
	SMTPAddress             string
	SMTPFrom                string
	SMTPUsername            string
	SMTPPassword            string
	InvitationLinkBaseURL   string
	NATSURL                 string
	OutboxInterval          time.Duration
}

func Load(requireJWTKey bool) (Config, error) {
	cfg := Config{
		HTTPAddress:            env("HTTP_ADDRESS", ":8082"),
		DatabaseURL:            env("DATABASE_URL", "postgres://reservehub:reservehub@localhost:5432/reservehub?sslmode=disable&search_path=auth_service"),
		BootstrapAdminEmail:    os.Getenv("AUTH_BOOTSTRAP_ADMIN_EMAIL"),
		BootstrapAdminPassword: os.Getenv("AUTH_BOOTSTRAP_ADMIN_PASSWORD"),
		TokenTTL:               15 * time.Minute,
		RefreshTTL:             30 * 24 * time.Hour,
		InvitationTTL:          48 * time.Hour,
		NATSURL:                os.Getenv("NATS_URL"),
		OutboxInterval:         time.Second,
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
		if tokenPath := os.Getenv("AUTH_PROVISION_SERVICE_TOKEN_FILE"); tokenPath != "" {
			encoded, err := os.ReadFile(tokenPath)
			if err != nil {
				return Config{}, fmt.Errorf("read Auth provisioning service token: %w", err)
			}
			cfg.ProvisionServiceToken = strings.TrimRight(string(encoded), "\r\n")
			if len(cfg.ProvisionServiceToken) < 32 || len(cfg.ProvisionServiceToken) > 4096 {
				return Config{}, fmt.Errorf("Auth provisioning service token must contain 32 to 4096 bytes")
			}
		}
	}
	if (cfg.BootstrapAdminEmail == "") != (cfg.BootstrapAdminPassword == "") {
		return Config{}, fmt.Errorf("AUTH_BOOTSTRAP_ADMIN_EMAIL and AUTH_BOOTSTRAP_ADMIN_PASSWORD must be set together")
	}
	if value := os.Getenv("AUTH_REFRESH_TTL"); value != "" {
		ttl, err := time.ParseDuration(value)
		if err != nil || ttl <= 0 || ttl > 90*24*time.Hour {
			return Config{}, fmt.Errorf("AUTH_REFRESH_TTL must be positive and at most 90 days")
		}
		cfg.RefreshTTL = ttl
	}
	if value := os.Getenv("AUTH_INVITATION_TTL"); value != "" {
		ttl, err := time.ParseDuration(value)
		if err != nil || ttl < time.Minute || ttl > 30*24*time.Hour {
			return Config{}, fmt.Errorf("AUTH_INVITATION_TTL must be between 1 minute and 30 days")
		}
		cfg.InvitationTTL = ttl
	}
	if value := os.Getenv("OUTBOX_INTERVAL"); value != "" {
		interval, err := time.ParseDuration(value)
		if err != nil || interval < 100*time.Millisecond {
			return Config{}, fmt.Errorf("OUTBOX_INTERVAL must be at least 100ms")
		}
		cfg.OutboxInterval = interval
	}
	if requireJWTKey {
		cfg.SMTPAddress = os.Getenv("AUTH_SMTP_ADDRESS")
		if cfg.SMTPAddress != "" {
			cfg.SMTPFrom = os.Getenv("AUTH_SMTP_FROM")
			cfg.SMTPUsername = os.Getenv("AUTH_SMTP_USERNAME")
			cfg.InvitationLinkBaseURL = os.Getenv("AUTH_INVITATION_LINK_BASE_URL")
			passwordPath := os.Getenv("AUTH_SMTP_PASSWORD_FILE")
			keyPath := os.Getenv("AUTH_INVITATION_ENCRYPTION_KEY_FILE")
			if passwordPath == "" || keyPath == "" {
				return Config{}, fmt.Errorf("SMTP password and invitation encryption key files are required when SMTP is enabled")
			}
			password, err := os.ReadFile(passwordPath)
			if err != nil {
				return Config{}, fmt.Errorf("read SMTP password: %w", err)
			}
			cfg.SMTPPassword = strings.TrimRight(string(password), "\r\n")
			encodedKey, err := os.ReadFile(keyPath)
			if err != nil {
				return Config{}, fmt.Errorf("read invitation encryption key: %w", err)
			}
			cfg.InvitationEncryptionKey, err = hex.DecodeString(strings.TrimSpace(string(encodedKey)))
			if err != nil || len(cfg.InvitationEncryptionKey) != 32 {
				return Config{}, fmt.Errorf("invitation encryption key file must contain 64 hex digits")
			}
		}
	}
	return cfg, nil
}

func env(name, fallback string) string {
	if value, ok := os.LookupEnv(name); ok {
		return value
	}
	return fallback
}
