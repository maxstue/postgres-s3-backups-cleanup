# Backup Cleanup

Go job that inventories a dedicated S3-compatible backup bucket, preserves the
newest `KEEP_MIN` objects, and deletes other objects older than
`RETENTION_DAYS`. It runs once and exits; Railway Cron controls the schedule.

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

All values are configured through service variables. Invalid settings fail before
contacting storage. Review preview logs before setting `DRY_RUN=false`.

Keep the bucket dedicated to backups. Retention applies to every object with a
key and modification time. All pages are listed before deletion, and the minimum
applies across the entire bucket. Objects exactly on the retention boundary are
preserved. A listing failure stops the run before any deletion.

From this directory:

```sh
go build ./cmd/cleanup
go vet ./...
go test ./...
```

Railway root directory: `/cleanup`. No custom start command is needed; the
[Dockerfile](Dockerfile) provides the entrypoint.
See the [shared overview](../README.md).
