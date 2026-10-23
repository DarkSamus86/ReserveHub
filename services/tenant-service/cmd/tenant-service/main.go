package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	tenantv1 "darksamus.com/ReserveHub/tenant-service/gen/tenant/v1"
	"darksamus.com/ReserveHub/tenant-service/internal/application"
	"darksamus.com/ReserveHub/tenant-service/internal/authn"
	"darksamus.com/ReserveHub/tenant-service/internal/config"
	"darksamus.com/ReserveHub/tenant-service/internal/messaging"
	"darksamus.com/ReserveHub/tenant-service/internal/observability"
	"darksamus.com/ReserveHub/tenant-service/internal/repository"
	transportgrpc "darksamus.com/ReserveHub/tenant-service/internal/transport/grpc"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

func main() {
	logger := observability.DefaultLogger("tenant-service", os.Getenv("APP_ENV"), os.Getenv("LOG_LEVEL"))
	slog.SetDefault(logger)
	if err := run(); err != nil {
		slog.Error("tenant-service stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logger := slog.Default()

	rootCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdownTracing, err := observability.InitTracing(rootCtx, cfg.OTLPEndpoint)
	if err != nil {
		return fmt.Errorf("initialize tracing: %w", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := shutdownTracing(ctx); err != nil {
			logger.Warn("failed to shut down tracing", "error", err)
		}
	}()

	store, err := repository.NewPostgres(rootCtx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer store.Close()
	if err := store.Ping(rootCtx); err != nil {
		return fmt.Errorf("database is unavailable: %w", err)
	}

	listener, err := net.Listen("tcp", cfg.GRPCAddress)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.GRPCAddress, err)
	}
	metrics := observability.NewMetrics()
	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(
			observability.UnaryInterceptor(metrics, logger),
			authn.UnaryServerInterceptor([]byte(cfg.AuthJWTSecret)),
		),
	)
	tenantService := application.New(store)
	tenantv1.RegisterTenantServiceServer(grpcServer, transportgrpc.New(tenantService))
	healthServer := health.NewServer()
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus("reservehub.tenant.v1.TenantService", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	reflection.Register(grpcServer)

	metricsServer := newMetricsServer(cfg.MetricsAddress, metrics, store)

	var publisher *messaging.Publisher
	if cfg.NATSURL != "" {
		publisher, err = messaging.NewPublisher(store, cfg.NATSURL, cfg.OutboxInterval, logger)
		if err != nil {
			return err
		}
		defer publisher.Close()
		go publisher.Run(rootCtx)
	} else {
		logger.Warn("NATS_URL is empty; transactional outbox publisher is disabled")
	}

	errCh := make(chan error, 2)
	go func() {
		logger.Info("gRPC server started", "address", cfg.GRPCAddress)
		if err := grpcServer.Serve(listener); err != nil {
			errCh <- fmt.Errorf("serve gRPC: %w", err)
		}
	}()
	go func() {
		logger.Info("metrics server started", "address", cfg.MetricsAddress)
		if err := metricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("serve metrics: %w", err)
		}
	}()

	var runErr error
	select {
	case <-rootCtx.Done():
		logger.Info("shutdown signal received")
	case runErr = <-errCh:
		logger.Error("server failed", "error", runErr)
		stop()
	}

	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	healthServer.SetServingStatus("reservehub.tenant.v1.TenantService", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	grpcServer.GracefulStop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		runErr = errors.Join(runErr, fmt.Errorf("shut down metrics server: %w", err))
	}
	return runErr
}

type pinger interface{ Ping(context.Context) error }

func newMetricsServer(address string, metrics *observability.Metrics, database pinger) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("/health/live", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/health/ready", func(w http.ResponseWriter, req *http.Request) {
		ctx, cancel := context.WithTimeout(req.Context(), time.Second)
		defer cancel()
		if err := database.Ping(ctx); err != nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return &http.Server{Addr: address, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
}
