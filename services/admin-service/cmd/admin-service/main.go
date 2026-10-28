package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"darksamus.com/ReserveHub/admin-service/internal/config"
	"darksamus.com/ReserveHub/admin-service/internal/httpapi"
	"darksamus.com/ReserveHub/admin-service/internal/messaging"
	"darksamus.com/ReserveHub/admin-service/internal/provision"
	"darksamus.com/ReserveHub/admin-service/internal/repository"
	tenantv1 "darksamus.com/ReserveHub/tenant-service/gen/tenant/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	if err := run(); err != nil {
		slog.Error("admin service stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	store, err := repository.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer store.Close()
	publisher, err := messaging.NewPublisher(store, cfg.NATSURL, cfg.OutboxInterval, slog.Default())
	if err != nil {
		return err
	}
	defer publisher.Close()
	go publisher.Run(ctx)

	conn, err := grpc.NewClient(cfg.TenantAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()

	service := provision.New(store, tenantv1.NewTenantServiceClient(conn), cfg.AuthServiceURL, cfg.ServiceToken)
	server := &http.Server{
		Addr:              cfg.HTTPAddress,
		Handler:           httpapi.New(store, service, cfg.PublicKey),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	var serveErr error
	select {
	case <-ctx.Done():
	case serveErr = <-errCh:
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return errors.Join(serveErr, server.Shutdown(shutdownCtx))
}
