package observability

import (
	"context"
	"os"
	"runtime/debug"

	"github.com/Mohith1612/qr-dining/internal/config"
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/semconv/v1.39.0"
)

// SetupTracing initializes the global OTel TracerProvider. When cfg.Enabled is
// false it installs nothing and returns a no-op shutdown.
func SetupTracing(ctx context.Context, cfg config.OTelConfig, logger zerolog.Logger) (func(context.Context) error, error) {
	if !cfg.Enabled {
		return func(context.Context) error { return nil }, nil
	}

	attrs := []resource.Option{
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.DeploymentEnvironmentName(os.Getenv("GIN_MODE")),
		),
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		attrs = append(attrs, resource.WithAttributes(semconv.ServiceVersion(info.Main.Version)))
	}
	res, err := resource.New(ctx, attrs...)
	if err != nil {
		return nil, err
	}
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.ExporterEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.TracesSampleRatio))),
		sdktrace.WithBatcher(exporter),
	)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		logger.Warn().Err(err).Msg("otel export error")
	}))

	return provider.Shutdown, nil
}
