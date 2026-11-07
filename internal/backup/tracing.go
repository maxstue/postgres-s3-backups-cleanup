package backup

import (
	"context"
	"net/url"

	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/smithy-go/middleware"
	"github.com/maxstue/postgres-s3-backups-cleanup/internal/backup/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Record operation names and the remote system only. Keys, bucket names,
// credentials, request URLs and SDK error text never enter exported spans.
func addS3Tracing(stack *middleware.Stack) error {
	return stack.Initialize.Add(middleware.InitializeMiddlewareFunc("BackupS3Tracing",
		func(ctx context.Context, input middleware.InitializeInput, next middleware.InitializeHandler) (middleware.InitializeOutput, middleware.Metadata, error) {
			operation := awsmiddleware.GetOperationName(ctx)
			attrs := []attribute.KeyValue{
				attribute.String("rpc.system", "aws-api"),
				attribute.String("rpc.service", "S3"),
				attribute.String("rpc.method", operation),
			}
			endpoint, _ := normalizeS3Endpoint(config.AWS.S3Endpoint)
			if u, err := url.Parse(endpoint); err == nil && u.Hostname() != "" {
				attrs = append(attrs, attribute.String("server.address", u.Hostname()))
			}
			ctx, span := otel.Tracer("postgres-s3-backups").Start(ctx, "S3 "+operation,
				trace.WithSpanKind(trace.SpanKindClient), trace.WithAttributes(attrs...))
			defer span.End()
			output, metadata, err := next.HandleInitialize(ctx, input)
			if err != nil {
				span.SetStatus(codes.Error, "")
			}
			return output, metadata, err
		}), middleware.After)
}
