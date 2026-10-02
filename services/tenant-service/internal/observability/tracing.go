package observability

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

func InitTracing(ctx context.Context, endpoint string) (func(context.Context) error, error) {
	options := []tracesdk.TracerProviderOption{
		tracesdk.WithResource(resource.NewWithAttributes(semconv.SchemaURL, semconv.ServiceName("tenant-service"))),
	}
	if endpoint != "" {
		exporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithEndpointURL(endpoint), otlptracegrpc.WithInsecure())
		if err != nil {
			return nil, err
		}
		options = append(options, tracesdk.WithBatcher(exporter))
	}
	provider := tracesdk.NewTracerProvider(options...)
	otel.SetTracerProvider(provider)
	return func(ctx context.Context) error {
		return provider.Shutdown(ctx)
	}, nil
}
