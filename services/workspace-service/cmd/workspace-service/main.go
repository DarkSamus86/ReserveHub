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

	providerv1 "darksamus.com/ReserveHub/provider-service/gen/provider/v1"
	"darksamus.com/ReserveHub/workspace-service/internal/config"
	"darksamus.com/ReserveHub/workspace-service/internal/httpapi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	if err := run(); err != nil {
		slog.Error("workspace service stopped", "error", err)
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
	conn, err := grpc.NewClient(cfg.ProviderAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()
	health := grpc_health_v1.NewHealthClient(conn)
	h := httpapi.Handler{Provider: providerv1.NewProviderServiceClient(conn), ProviderToken: cfg.ProviderServiceToken, PublicKey: cfg.PublicKey, Ready: func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		response, err := health.Check(ctx, &grpc_health_v1.HealthCheckRequest{Service: providerv1.ProviderService_ServiceDesc.ServiceName})
		if err != nil {
			return err
		}
		if response.GetStatus() != grpc_health_v1.HealthCheckResponse_SERVING {
			return errors.New("provider not serving")
		}
		return nil
	}}
	server := &http.Server{Addr: cfg.HTTPAddress, Handler: h.Routes(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second}
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
