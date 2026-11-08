# Cleanup

Inventories a dedicated S3-compatible backup bucket, preserves the newest
`KEEP_MIN` objects, and deletes other objects older than `RETENTION_DAYS`.
Runs once and exits; Railway Cron controls the schedule.

| Variable | Meaning |
| --- | --- |
| `RETENTION_DAYS` | Required positive integer; template default 183 |
| `KEEP_MIN` | Non-negative minimum to preserve; default 1 |
| `DRY_RUN` | Boolean; defaults to true, previews without deleting |
| `ACCESS_KEY_ID` | Required bucket access key |
| `SECRET_ACCESS_KEY` | Required bucket secret |
| `BUCKET` | Required bucket name |
| `ENDPOINT` | Required HTTP(S) endpoint or hostname |
| `REGION` | Bucket region; default auto |
| `AWS_S3_FORCE_PATH_STYLE` | Boolean; default true |

Invalid settings fail before contacting storage. Review preview logs before
setting `DRY_RUN=false`.

Keep the bucket dedicated to backups. Retention applies to every object with a
key and modification time. All pages are listed before deletion, and the minimum
applies across the entire bucket. Objects exactly on the retention boundary are
preserved. A listing failure stops the run before any deletion.

## Build and run

From the repository root:

```sh
go build -o bin/cleanup ./cmd/cleanup
go test ./internal/cleanup/...
docker build -f docker/cleanup.Dockerfile -t postgres-s3-cleanup .
```

The Railway template uses a prebuilt GHCR image; no root directory or Dockerfile
variable is needed. For custom GitHub-source builds, use root directory `/` and
`RAILWAY_DOCKERFILE_PATH=docker/cleanup.Dockerfile`.
The image entrypoint starts the cleanup binary.

See the [repository overview](../../README.md).
