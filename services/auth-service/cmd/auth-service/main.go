package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"darksamus.com/ReserveHub/auth-service/internal/auth"
	"darksamus.com/ReserveHub/auth-service/internal/config"
	"darksamus.com/ReserveHub/auth-service/internal/delivery"
	"darksamus.com/ReserveHub/auth-service/internal/httpapi"
	"darksamus.com/ReserveHub/auth-service/internal/messaging"
	"darksamus.com/ReserveHub/auth-service/internal/observability"
	"darksamus.com/ReserveHub/auth-service/internal/repository"
)

func main() {
	logger := observability.DefaultLogger("auth-service", os.Getenv("APP_ENV"), os.Getenv("LOG_LEVEL"))
	slog.SetDefault(logger)
	if err := run(); err != nil {
		slog.Error("auth_service_stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	bootstrapMode := len(os.Args) == 2 && os.Args[1] == "bootstrap-admin"
	if len(os.Args) != 1 && !bootstrapMode {
		return fmt.Errorf("usage: auth-service [bootstrap-admin]")
	}
	cfg, err := config.Load(!bootstrapMode)
	if err != nil {
		return err
	}
	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	store, err := repository.NewPostgres(rootCtx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer store.Close()

	service := auth.NewService(store, cfg.JWTPrivateKey, cfg.TokenTTL)
	service.SetRefreshTTL(cfg.RefreshTTL)
	if bootstrapMode {
		if err := service.BootstrapAdmin(rootCtx, cfg.BootstrapAdminEmail, cfg.BootstrapAdminPassword); err != nil {
			return fmt.Errorf("bootstrap admin: %w", err)
		}
		slog.Info("bootstrap_admin_ready")
		return nil
	}
	hasAdmin, err := store.HasAdmin(rootCtx)
	if err != nil {
		return fmt.Errorf("check for administrator: %w", err)
	}
	if !hasAdmin {
		return fmt.Errorf("no administrator exists; run auth-service bootstrap-admin with AUTH_BOOTSTRAP_ADMIN_EMAIL and AUTH_BOOTSTRAP_ADMIN_PASSWORD before starting the server")
	}
	if cfg.NATSURL == "" {
		return fmt.Errorf("NATS_URL is required")
	}
	publisher, err := messaging.NewPublisher(store, cfg.NATSURL, cfg.OutboxInterval, slog.Default())
	if err != nil {
		return err
	}
	defer publisher.Close()
	go publisher.Run(rootCtx)
	var deliveryErrCh <-chan error
	if cfg.SMTPAddress != "" {
		sender, err := delivery.NewSMTP(cfg.SMTPAddress, cfg.SMTPFrom, cfg.SMTPUsername, cfg.SMTPPassword, cfg.InvitationLinkBaseURL)
		if err != nil {
			return err
		}
		ch := make(chan error, 1)
		deliveryErrCh = ch
		go func() {
			ch <- store.RunInvitationDelivery(rootCtx, cfg.InvitationEncryptionKey, sender, cfg.InvitationTTL)
		}()
	}

	server := &http.Server{
		Addr: cfg.HTTPAddress, Handler: httpapi.New(service, slog.Default(), cfg.ProvisionServiceToken),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		slog.Info("http_server_started", "address", cfg.HTTPAddress)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("serve HTTP: %w", err)
		}
	}()

	var runErr error
	select {
	case <-rootCtx.Done():
		slog.Info("shutdown_signal_received")
	case runErr = <-errCh:
		stop()
	case runErr = <-deliveryErrCh:
		stop()
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		runErr = errors.Join(runErr, fmt.Errorf("shut down HTTP server: %w", err))
	}
	return runErr
}
