package telemetry

import (
	"context"
	"errors"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Setup exports traces only when an OTLP endpoint is configured. Resource
// identity and exporter credentials remain controlled by the hosting platform.
func Setup(ctx context.Context) (func(context.Context) error, error) {
	noop := func(context.Context) error { return nil }
	if os.Getenv("OTEL_SDK_DISABLED") == "true" || os.Getenv("OTEL_TRACES_EXPORTER") == "none" ||
		(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" && os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") == "") {
		return noop, nil
	}
	protocol := os.Getenv("OTEL_EXPORTER_OTLP_TRACES_PROTOCOL")
	if protocol == "" {
		protocol = os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL")
	}
	var exporter sdktrace.SpanExporter
	var err error
	switch protocol {
	case "", "http/protobuf":
		exporter, err = otlptracehttp.New(ctx)
	case "grpc":
		exporter, err = otlptracegrpc.New(ctx)
	default:
		return nil, errors.New("unsupported OTLP traces protocol")
	}
	if err != nil {
		return nil, errors.New("could not initialize trace exporter")
	}
	attrs := []attribute.KeyValue{attribute.String("service.name", "postgres-s3-backups")}
	if version := os.Getenv("OTEL_SERVICE_VERSION"); version != "" {
		attrs = append(attrs, attribute.String("service.version", version))
	}
	res, err := resource.New(ctx, resource.WithAttributes(attrs...), resource.WithFromEnv())
	if err != nil {
		_ = exporter.Shutdown(ctx)
		return nil, errors.New("could not initialize trace resource")
	}
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exporter, sdktrace.WithExportTimeout(5*time.Second)),
	)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return provider.Shutdown, nil
}
