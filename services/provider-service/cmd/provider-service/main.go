package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	providerv1 "darksamus.com/ReserveHub/provider-service/gen/provider/v1"
	"darksamus.com/ReserveHub/provider-service/internal/application"
	"darksamus.com/ReserveHub/provider-service/internal/config"
	"darksamus.com/ReserveHub/provider-service/internal/grpcapi"
	"darksamus.com/ReserveHub/provider-service/internal/messaging"
	"darksamus.com/ReserveHub/provider-service/internal/repository"
	"darksamus.com/ReserveHub/provider-service/internal/tenantclient"
	tenantv1 "darksamus.com/ReserveHub/tenant-service/gen/tenant/v1"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
	if err := run(); err != nil {
		slog.Error("provider service stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err = pool.Ping(ctx); err != nil {
		return err
	}
	store := repository.Postgres{Pool: pool}
	publisher, err := messaging.NewPublisher(store, cfg.NATSURL, cfg.OutboxInterval, slog.Default())
	if err != nil {
		return err
	}
	defer publisher.Close()
	go publisher.Run(ctx)

	tenantConn, err := grpc.NewClient(cfg.TenantAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer tenantConn.Close()
	service := application.Service{Store: store, Tenant: tenantclient.NewLookup(tenantv1.NewTenantServiceClient(tenantConn), cfg.TenantServiceToken)}

	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(grpcapi.UnaryAuth(cfg.PublicKey, cfg.AdminServiceToken, cfg.WorkspaceServiceToken)), grpc.MaxRecvMsgSize(1<<20))
	providerv1.RegisterProviderServiceServer(grpcServer, grpcapi.Server{Service: service})
	healthServer := health.NewServer()
	setHealthStatus(healthServer, grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	go monitorHealth(ctx, pool, healthServer)
	listener, err := net.Listen("tcp", cfg.GRPCAddress)
	if err != nil {
		return err
	}
	metrics := &http.Server{Addr: cfg.MetricsAddress, Handler: healthRoutes(pool), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second}

	errCh := make(chan error, 2)
	go func() {
		if e := grpcServer.Serve(listener); e != nil {
			errCh <- e
		}
	}()
	go func() {
		if e := metrics.ListenAndServe(); e != nil && !errors.Is(e, http.ErrServerClosed) {
			errCh <- e
		}
	}()
	var serveErr error
	select {
	case <-ctx.Done():
	case serveErr = <-errCh:
	}
	setHealthStatus(healthServer, grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	grpcStopped := make(chan struct{})
	go func() { grpcServer.GracefulStop(); close(grpcStopped) }()
	select {
	case <-grpcStopped:
	case <-shutdownCtx.Done():
		grpcServer.Stop()
	}
	return errors.Join(serveErr, metrics.Shutdown(shutdownCtx))
}

func healthRoutes(pool *pgxpool.Pool) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

func monitorHealth(ctx context.Context, pool *pgxpool.Pool, server *health.Server) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			setHealthStatus(server, grpc_health_v1.HealthCheckResponse_NOT_SERVING)
			return
		case <-ticker.C:
			probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			err := pool.Ping(probeCtx)
			cancel()
			if err != nil {
				setHealthStatus(server, grpc_health_v1.HealthCheckResponse_NOT_SERVING)
			} else {
				setHealthStatus(server, grpc_health_v1.HealthCheckResponse_SERVING)
			}
		}
	}
}

func setHealthStatus(server *health.Server, state grpc_health_v1.HealthCheckResponse_ServingStatus) {
	server.SetServingStatus("", state)
	server.SetServingStatus(providerv1.ProviderService_ServiceDesc.ServiceName, state)
}
