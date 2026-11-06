package config

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	GRPCAddress           string
	MetricsAddress        string
	DatabaseURL           string
	TenantAddress         string
	PublicKey             ed25519.PublicKey
	TenantServiceToken    string
	AdminServiceToken     string
	WorkspaceServiceToken string
	NATSURL               string
	OutboxInterval        time.Duration
}

func LoadConfig() (Config, error) {
	c := Config{GRPCAddress: os.Getenv("GRPC_ADDRESS"), MetricsAddress: os.Getenv("METRICS_ADDRESS"), DatabaseURL: os.Getenv("DATABASE_URL"), TenantAddress: os.Getenv("TENANT_GRPC_ADDRESS"), NATSURL: os.Getenv("NATS_URL"), OutboxInterval: time.Second}
	if c.GRPCAddress == "" {
		c.GRPCAddress = ":8084"
	}
	if c.MetricsAddress == "" {
		c.MetricsAddress = ":9094"
	}
	if c.DatabaseURL == "" || c.TenantAddress == "" || c.NATSURL == "" {
		return c, errors.New("DATABASE_URL, TENANT_GRPC_ADDRESS and NATS_URL are required")
	}
	if s := os.Getenv("OUTBOX_INTERVAL"); s != "" {
		d, err := time.ParseDuration(s)
		if err != nil || d <= 0 {
			return c, errors.New("OUTBOX_INTERVAL must be a positive duration")
		}
		c.OutboxInterval = d
	}
	path := os.Getenv("AUTH_JWT_PUBLIC_KEY_FILE")
	if path == "" {
		return c, errors.New("AUTH_JWT_PUBLIC_KEY_FILE is required")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return c, fmt.Errorf("read public key: %w", err)
	}
	block, rest := pem.Decode(b)
	if block == nil || len(strings.TrimSpace(string(rest))) != 0 {
		return c, errors.New("invalid public key PEM")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return c, fmt.Errorf("parse public key: %w", err)
	}
	key, ok := parsed.(ed25519.PublicKey)
	if !ok || len(key) != ed25519.PublicKeySize {
		return c, errors.New("public key must be Ed25519")
	}
	c.PublicKey = key
	c.TenantServiceToken, err = readToken("PLATFORM_SERVICE_TOKEN_FILE")
	if err != nil {
		return c, err
	}
	c.AdminServiceToken, err = readToken("PROVIDER_ADMIN_SERVICE_TOKEN_FILE")
	if err != nil {
		return c, err
	}
	c.WorkspaceServiceToken, err = readToken("PROVIDER_WORKSPACE_SERVICE_TOKEN_FILE")
	if err != nil {
		return c, err
	}
	if c.AdminServiceToken == c.WorkspaceServiceToken || c.AdminServiceToken == c.TenantServiceToken || c.WorkspaceServiceToken == c.TenantServiceToken {
		return c, errors.New("Provider caller tokens must be distinct from each other and the Tenant service token")
	}
	return c, nil
}

func readToken(name string) (string, error) {
	path := os.Getenv(name)
	if path == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	encoded, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", name, err)
	}
	token := strings.TrimSpace(string(encoded))
	if len(token) < 32 {
		return "", fmt.Errorf("%s is too short", name)
	}
	return token, nil
}
