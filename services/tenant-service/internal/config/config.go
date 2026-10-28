package config

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AuthJWTPublicKey     ed25519.PublicKey
	PlatformServiceToken []byte
	GRPCAddress          string
	MetricsAddress       string
	DatabaseURL          string
	NATSURL              string
	OTLPEndpoint         string
	ShutdownTimeout      time.Duration
	OutboxInterval       time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		GRPCAddress:    env("GRPC_ADDRESS", ":8081"),
		MetricsAddress: env("METRICS_ADDRESS", ":9091"),
		DatabaseURL:    env("DATABASE_URL", "postgres://reservehub:reservehub@localhost:5432/reservehub?sslmode=disable&search_path=tenant_service"),
		NATSURL:        env("NATS_URL", ""),
		OTLPEndpoint:   env("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
	}

	path := env("AUTH_JWT_PUBLIC_KEY_FILE", "")
	if path == "" {
		return Config{}, fmt.Errorf("AUTH_JWT_PUBLIC_KEY_FILE is required")
	}
	encoded, err := os.ReadFile(path)
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
	cfg.AuthJWTPublicKey = append(ed25519.PublicKey(nil), key...)
	serviceTokenFile := os.Getenv("PLATFORM_SERVICE_TOKEN_FILE")
	if serviceTokenFile == "" {
		return Config{}, fmt.Errorf("PLATFORM_SERVICE_TOKEN_FILE is required")
	}
	encodedServiceToken, err := os.ReadFile(serviceTokenFile)
	if err != nil {
		return Config{}, fmt.Errorf("read platform service token: %w", err)
	}
	cfg.PlatformServiceToken = []byte(strings.TrimSpace(string(encodedServiceToken)))
	if len(cfg.PlatformServiceToken) < 32 {
		return Config{}, fmt.Errorf("platform service token must be at least 32 bytes")
	}

	if cfg.ShutdownTimeout, err = duration("SHUTDOWN_TIMEOUT", 10*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.OutboxInterval, err = duration("OUTBOX_INTERVAL", time.Second); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func env(name, fallback string) string {
	if value, ok := os.LookupEnv(name); ok {
		return value
	}
	return fallback
}

func duration(name string, fallback time.Duration) (time.Duration, error) {
	value, ok := os.LookupEnv(name)
	if !ok {
		return fallback, nil
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		return time.Duration(seconds) * time.Second, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", name, err)
	}
	return parsed, nil
}
