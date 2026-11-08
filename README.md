# Postgres S3 Backups with Cleanup

[![Deploy on Railway](https://railway.com/button.svg)](https://railway.com/new/template/rpjicw)

Two independently built Go applications share a dedicated S3-compatible backup
bucket. The repository uses one Go module with separate commands and application
packages.

| Application | Entrypoint | Implementation | Dockerfile |
| --- | --- | --- | --- |
| Backup | [cmd/backup](cmd/backup/) | [internal/backup](internal/backup/) | [docker/backup.Dockerfile](docker/backup.Dockerfile) |
| Cleanup | [cmd/cleanup](cmd/cleanup/) | [internal/cleanup](internal/cleanup/) | [docker/cleanup.Dockerfile](docker/cleanup.Dockerfile) |

Backup streams a PostgreSQL dump through parallel gzip compression directly to
S3. Cleanup previews or deletes expired backups while preserving a configurable
minimum number. Each application has its own binary, image, configuration and
schedule. Neither application's image contains the other application's binary.

## Railway template

Deploy into the project containing your existing PostgreSQL database and provide
`BACKUP_DATABASE_URL`, for example `${{Postgres.DATABASE_URL}}`. No database is
created. Railway creates a dedicated bucket and supplies both services with
generated reference variables.

Both services build from the repository root (`/`). Their
`RAILWAY_DOCKERFILE_PATH` variables select the corresponding Dockerfile.

| Setting | Template default | Where to change it |
| --- | --- | --- |
| Backup schedule | `0 2 1 * *` (monthly, day 1 at 02:00 UTC) | Backup CRON → Settings |
| Cleanup schedule | `0 3 1 1,7 *` (January/July, day 1 at 03:00 UTC) | Backup Cleanup → Settings |
| `RUN_ON_STARTUP` | `true` | Backup CRON → Variables |
| `GZIP_CONCURRENCY` | `20` | Backup CRON → Variables |
| `UPLOAD_CONCURRENCY` | `20` | Backup CRON → Variables |
| `PG_VERSION` | `18` (at least your database server major version) | Backup CRON → Variables |
| `RETENTION_DAYS` | `183` | Backup Cleanup → Variables |
| `KEEP_MIN` | `1` | Backup Cleanup → Variables |
| `DRY_RUN` | `true` | Backup Cleanup → Variables |

All defaults are configurable. Native Railway cron schedules use UTC. Both
applications run once and exit; Railway starts each scheduled run.

Cleanup starts in preview mode. Review the logs, then set `DRY_RUN=false` to
enable deletion. Keep the bucket dedicated to backups: retention applies to all
its objects. Regularly test restoring backups.

For external S3 storage, update bucket, endpoint, region and credentials on both
services. The template creates a Railway bucket by default.

## Development

Run all checks from the repository root; no database or cloud credentials are
required for the tests.

```sh
go test ./...
go vet ./...
```

Build the applications independently:

```sh
go build -o bin/backup ./cmd/backup
go build -o bin/cleanup ./cmd/cleanup

docker build --build-arg PG_VERSION=18 -f docker/backup.Dockerfile -t postgres-s3-backup .
docker build -f docker/cleanup.Dockerfile -t postgres-s3-cleanup .
```

See [backup configuration](cmd/backup/README.md) and
[cleanup configuration](cmd/cleanup/README.md) for the full variable reference.

## Acknowledgements

Thanks to the maintainers and contributors of [postgres-s3-backups](https://github.com/railwayapp-templates/postgres-s3-backups) for the foundation of this project.

## Marketplace publication

In the Railway template editor, open **Marketplace**, review the listing, apply
any pending changes, then click **Publish to marketplace**.
See [Railway's guide](https://docs.railway.com/templates/publish-and-share).
