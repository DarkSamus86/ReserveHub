package config

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	HTTPAddress          string
	ProviderAddress      string
	PublicKey            ed25519.PublicKey
	ProviderServiceToken string
}

func Load() (Config, error) {
	cfg := Config{HTTPAddress: os.Getenv("HTTP_ADDRESS"), ProviderAddress: os.Getenv("PROVIDER_GRPC_ADDRESS")}
	if cfg.HTTPAddress == "" {
		cfg.HTTPAddress = ":8085"
	}
	if cfg.ProviderAddress == "" {
		return Config{}, errors.New("PROVIDER_GRPC_ADDRESS is required")
	}
	keyPath := os.Getenv("AUTH_JWT_PUBLIC_KEY_FILE")
	if keyPath == "" {
		return Config{}, errors.New("AUTH_JWT_PUBLIC_KEY_FILE is required")
	}
	encoded, err := os.ReadFile(keyPath)
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
	tokenPath := os.Getenv("PROVIDER_WORKSPACE_SERVICE_TOKEN_FILE")
	if tokenPath == "" {
		return Config{}, errors.New("PROVIDER_WORKSPACE_SERVICE_TOKEN_FILE is required")
	}
	encoded, err = os.ReadFile(tokenPath)
	if err != nil {
		return Config{}, fmt.Errorf("read Provider workload token: %w", err)
	}
	cfg.ProviderServiceToken = strings.TrimSpace(string(encoded))
	if len(cfg.ProviderServiceToken) < 32 {
		return Config{}, errors.New("Provider workload token too short")
	}
	return cfg, nil
}
