package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	GRPCAddress     string
	MetricsAddress  string
	DatabaseURL     string
	NATSURL         string
	OTLPEndpoint    string
	ShutdownTimeout time.Duration
	OutboxInterval  time.Duration
	LogLevel        string
}

func Load() (Config, error) {
	cfg := Config{
		GRPCAddress:    env("GRPC_ADDRESS", ":8081"),
		MetricsAddress: env("METRICS_ADDRESS", ":9091"),
		DatabaseURL:    env("DATABASE_URL", "postgres://reservehub:reservehub@localhost:5432/reservehub?sslmode=disable&search_path=tenant_service"),
		NATSURL:        env("NATS_URL", ""),
		OTLPEndpoint:   env("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
		LogLevel:       env("LOG_LEVEL", "info"),
	}

	var err error
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
