package observability

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type Metrics struct {
	Requests *prometheus.CounterVec
	Duration *prometheus.HistogramVec
	Registry *prometheus.Registry
}

func NewMetrics() *Metrics {
	registry := prometheus.NewRegistry()
	metrics := &Metrics{
		Requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "reservehub", Subsystem: "tenant_service", Name: "grpc_requests_total",
			Help: "Total number of handled gRPC requests.",
		}, []string{"method", "code"}),
		Duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "reservehub", Subsystem: "tenant_service", Name: "grpc_request_duration_seconds",
			Help: "Duration of gRPC requests in seconds.",
		}, []string{"method"}),
		Registry: registry,
	}
	registry.MustRegister(metrics.Requests, metrics.Duration, prometheus.NewGoCollector(), prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	return metrics
}

func UnaryInterceptor(metrics *Metrics, logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		started := time.Now()
		requestID := requestID(ctx)
		_ = grpc.SetHeader(ctx, metadata.Pairs("x-request-id", requestID))
		response, err := handler(ctx, req)
		code := status.Code(err).String()
		metrics.Requests.WithLabelValues(info.FullMethod, code).Inc()
		metrics.Duration.WithLabelValues(info.FullMethod).Observe(time.Since(started).Seconds())
		logger.LogAttrs(ctx, slog.LevelInfo, "gRPC request",
			slog.String("request_id", requestID), slog.String("method", info.FullMethod),
			slog.String("code", code), slog.Duration("duration", time.Since(started)),
		)
		return response, err
	}
}

func requestID(ctx context.Context) string {
	if values := metadata.ValueFromIncomingContext(ctx, "x-request-id"); len(values) > 0 && values[0] != "" {
		return values[0]
	}
	return uuid.NewString()
}
