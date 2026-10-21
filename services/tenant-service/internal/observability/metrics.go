package observability

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
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
		duration := time.Since(started)
		metrics.Requests.WithLabelValues(info.FullMethod, code).Inc()
		metrics.Duration.WithLabelValues(info.FullMethod).Observe(duration.Seconds())
		level := slog.LevelInfo
		switch status.Code(err) {
		case codes.OK:
		case codes.Internal, codes.Unknown, codes.Unavailable, codes.DataLoss:
			level = slog.LevelError
		default:
			level = slog.LevelWarn
		}
		attrs := []slog.Attr{
			slog.String("request_id", requestID),
			slog.String("grpc_method", info.FullMethod),
			slog.String("grpc_code", code),
			slog.Float64("duration_ms", float64(duration)/float64(time.Millisecond)),
		}
		if tenantRequest, ok := req.(interface{ GetTenantId() string }); ok && tenantRequest.GetTenantId() != "" {
			attrs = append(attrs, slog.String("tenant_id", tenantRequest.GetTenantId()))
		}
		span := trace.SpanContextFromContext(ctx)
		if span.IsValid() {
			attrs = append(attrs, slog.String("trace_id", span.TraceID().String()), slog.String("span_id", span.SpanID().String()))
		}
		logger.LogAttrs(ctx, level, "grpc_request", attrs...)
		return response, err
	}
}

func requestID(ctx context.Context) string {
	if values := metadata.ValueFromIncomingContext(ctx, "x-request-id"); len(values) > 0 && values[0] != "" {
		return values[0]
	}
	return uuid.NewString()
}
