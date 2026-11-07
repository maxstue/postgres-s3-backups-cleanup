package telemetry

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	collector "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/proto"
)

func TestSetupFlushesTraceWithPlatformIdentity(t *testing.T) {
	previous := otel.GetTracerProvider()
	t.Cleanup(func() { otel.SetTracerProvider(previous) })
	requests := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requests <- body
		w.Header().Set("Content-Type", "application/x-protobuf")
	}))
	defer server.Close()
	for _, key := range []string{"OTEL_SDK_DISABLED", "OTEL_TRACES_EXPORTER", "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "OTEL_EXPORTER_OTLP_TRACES_PROTOCOL", "OTEL_EXPORTER_OTLP_HEADERS", "OTEL_EXPORTER_OTLP_TRACES_HEADERS", "OTEL_RESOURCE_ATTRIBUTES"} {
		t.Setenv(key, "")
	}
	t.Setenv("OTEL_SERVICE_NAME", "platform-backup")
	t.Setenv("OTEL_SERVICE_VERSION", "test-version")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", server.URL)
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "http/protobuf")
	shutdown, err := Setup(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	_, span := otel.Tracer("test").Start(context.Background(), "backup.run")
	span.SetStatus(codes.Error, "")
	span.End()
	if err := shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case data := <-requests:
		var request collector.ExportTraceServiceRequest
		if err := proto.Unmarshal(data, &request); err != nil {
			t.Fatal(err)
		}
		if len(request.ResourceSpans) != 1 {
			t.Fatalf("incorrect resource spans: %v", &request)
		}
		attrs := map[string]string{}
		for _, attr := range request.ResourceSpans[0].Resource.Attributes {
			attrs[attr.Key] = attr.Value.GetStringValue()
		}
		if attrs["service.name"] != "platform-backup" || attrs["service.version"] != "test-version" {
			t.Fatalf("incorrect identity: %v", attrs)
		}
		if len(request.ResourceSpans[0].ScopeSpans[0].Spans) != 1 {
			t.Fatal("error trace did not flush")
		}
	default:
		t.Fatal("trace not exported before shutdown returned")
	}
}
