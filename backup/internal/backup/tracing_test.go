package backup

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/maxstue/postgres-s3-backups-cleanup/backup/internal/config"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func traceRecorder(t *testing.T) *tracetest.InMemoryExporter {
	t.Helper()
	previous := otel.GetTracerProvider()
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
	otel.SetTracerProvider(provider)
	t.Cleanup(func() { otel.SetTracerProvider(previous); _ = provider.Shutdown(context.Background()) })
	return exporter
}

func TestDumpTraceParentAndPrivacy(t *testing.T) {
	configureBackupTest(t)
	exporter := traceRecorder(t)
	config.Backup.DatabaseURL = "postgresql://fixture-user:fixture-secret@db.internal/private-db?token=fixture-token"
	ctx, root := otel.Tracer("test").Start(context.Background(), "backup.run")
	_, _, err := streamBackup(ctx, backupTestCommand("success"), func(_ context.Context, _ string, body io.ReadCloser) (*uploadInfo, error) {
		_, err := io.Copy(io.Discard, body)
		return &uploadInfo{}, err
	})
	root.End()
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, span := range exporter.GetSpans() {
		if span.Name == "pg_dump" {
			found = true
			if span.Parent.SpanID() != root.SpanContext().SpanID() || span.SpanKind != trace.SpanKindClient {
				t.Fatalf("incorrect dump parent/kind: %+v", span)
			}
		}
		rendered := fmt.Sprint(span.Attributes, span.Events, span.Status)
		for _, secret := range []string{"fixture-user", "fixture-secret", "fixture-token", "postgresql://"} {
			if strings.Contains(rendered, secret) {
				t.Fatalf("trace contains %s", secret)
			}
		}
	}
	if !found {
		t.Fatal("missing pg_dump span")
	}
}

func TestS3MultipartTraceAndPathStyle(t *testing.T) {
	configureBackupTest(t)
	previous := *config.AWS
	t.Cleanup(func() { *config.AWS = previous })
	config.AWS.S3Endpoint = "https://storage.internal"
	config.AWS.S3Bucket = "private-bucket"
	config.AWS.ForcePathStyle = true
	exporter := traceRecorder(t)
	client, err := NewS3Client()
	if err != nil {
		t.Fatal(err)
	}
	options := client.Options()
	options.HTTPClient = endpointTestHTTPClient(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host != "storage.internal" || !strings.HasPrefix(request.URL.Path, "/private-bucket/") {
			t.Errorf("path-style request not preserved: %s", request.URL.Redacted())
		}
		if request.Body != nil {
			_, _ = io.Copy(io.Discard, request.Body)
		}
		body := ""
		if request.Method == "POST" && request.URL.Query().Has("uploads") {
			body = "<InitiateMultipartUploadResult><UploadId>test-upload</UploadId></InitiateMultipartUploadResult>"
		} else if request.Method == "POST" {
			body = "<CompleteMultipartUploadResult><ETag>test-etag</ETag></CompleteMultipartUploadResult>"
		}
		return &http.Response{StatusCode: http.StatusOK,
			Header: http.Header{"Content-Type": {"application/xml"}, "Etag": {"test-etag"}},
			Body:   io.NopCloser(strings.NewReader(body))}, nil
	})
	client = s3.New(options)
	ctx, root := otel.Tracer("test").Start(context.Background(), "backup.run")
	_, err = uploadToS3(ctx, client, "private-key.gz", io.NopCloser(bytes.NewReader(make([]byte, uploadPartSize+1))))
	root.End()
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]bool{"S3 CreateMultipartUpload": false, "S3 UploadPart": false, "S3 CompleteMultipartUpload": false}
	for _, span := range exporter.GetSpans() {
		if _, ok := expected[span.Name]; ok {
			expected[span.Name] = true
			if span.Parent.SpanID() != root.SpanContext().SpanID() || span.SpanKind != trace.SpanKindClient {
				t.Fatalf("incorrect S3 parent/kind: %+v", span)
			}
		}
		rendered := fmt.Sprint(span.Attributes, span.Events, span.Status)
		for _, secret := range []string{"private-key", "private-bucket", "Authorization", "fixture-secret"} {
			if strings.Contains(rendered, secret) {
				t.Fatalf("trace contains %s", secret)
			}
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("missing %s span", name)
		}
	}
}
