# Backup

Streams a PostgreSQL custom-format dump through parallel gzip compression into
concurrent S3 multipart uploads without storing the full archive on disk or in
memory.

| Variable | Meaning |
| --- | --- |
| `BACKUP_DATABASE_URL` | Required PostgreSQL connection URL |
| `AWS_ACCESS_KEY_ID` | Required S3 access key |
| `AWS_SECRET_ACCESS_KEY` | Required S3 secret |
| `AWS_S3_BUCKET` | Required bucket name |
| `AWS_S3_REGION` | Required region; use your provider's value |
| `AWS_S3_ENDPOINT` | Optional HTTP(S) endpoint or hostname; unset for standard AWS S3 |
| `AWS_S3_FORCE_PATH_STYLE` | Boolean, default false; template sets true |
| `RUN_ON_STARTUP` | Default false; template sets true to run once and exit |
| `GZIP_CONCURRENCY` | Positive compression worker count; default 20 |
| `UPLOAD_CONCURRENCY` | Positive upload worker count; default 20 |
| `PG_VERSION` | Docker build argument; template uses 17, at least the server version |

Compression blocks and upload parts are 5 MiB. More workers can improve throughput
but increase memory use. Credentials must permit listing, uploading and aborting
failed multipart uploads.

## OpenTelemetry

Enable Railway tracing after deployment. The SDK reads platform-supplied
`OTEL_EXPORTER_OTLP_*`, `OTEL_SERVICE_NAME`, and `OTEL_SERVICE_VERSION`, and
flushes before exit. HTTP/protobuf and gRPC are supported. No endpoint means no
exporter. `OTEL_SDK_DISABLED=true` or `OTEL_TRACES_EXPORTER=none` disables it.

Spans cover `backup.run`, `pg_dump`, and S3 operations including multipart
uploads. Credentials, object keys, bucket names, connection URLs and SDK error
text are omitted from span attributes/events. Only traces are exported.
Leave automatic instrumentation off.

## Build and run

From the repository root:

```sh
go build -o bin/backup ./cmd/backup
go test ./internal/backup/...
docker build --build-arg PG_VERSION=17 -f docker/backup.Dockerfile -t postgres-s3-backup .
```

Configuration is loaded explicitly when the application starts. Tests do not
require real database or S3 credentials.

Railway root directory: `/`.
`RAILWAY_DOCKERFILE_PATH=docker/backup.Dockerfile`.
The image entrypoint checks database readiness and starts the backup binary.
Use native Railway Cron for scheduling.

See the [repository overview](../../README.md).
