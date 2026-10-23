package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	HTTPAddress            string
	DatabaseURL            string
	JWTSecret              string
	TokenTTL               time.Duration
	BootstrapAdminEmail    string
	BootstrapAdminPassword string
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddress:            env("HTTP_ADDRESS", ":8082"),
		DatabaseURL:            env("DATABASE_URL", "postgres://reservehub:reservehub@localhost:5432/reservehub?sslmode=disable&search_path=auth_service"),
		JWTSecret:              os.Getenv("AUTH_JWT_SECRET"),
		BootstrapAdminEmail:    os.Getenv("AUTH_BOOTSTRAP_ADMIN_EMAIL"),
		BootstrapAdminPassword: os.Getenv("AUTH_BOOTSTRAP_ADMIN_PASSWORD"),
		TokenTTL:               15 * time.Minute,
	}
	if len(cfg.JWTSecret) < 32 {
		return Config{}, fmt.Errorf("AUTH_JWT_SECRET must contain at least 32 bytes")
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
