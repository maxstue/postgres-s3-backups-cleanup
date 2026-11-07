# Postgres S3 Backups with Cleanup

[![Deploy on Railway](https://railway.com/button.svg)](https://railway.com/new/template/rpjicw)

Two independent Go jobs share a dedicated S3-compatible backup bucket.

| Part | Source | Railway root directory |
| --- | --- | --- |
| Backup CRON | [backup/](backup/) | `/backup` |
| Backup Cleanup | [cleanup/](cleanup/) | `/cleanup` |
| Bucket | Railway template resource | — |

Each application has its own Go module, Dockerfile, command, internal packages
and tests. The backup streams a PostgreSQL dump through parallel gzip compression
directly to S3. The cleanup previews or deletes expired backups while preserving
a configurable minimum number.

## Railway template

Deploy into the project containing your existing PostgreSQL database and provide
`BACKUP_DATABASE_URL`, for example `${{Postgres.DATABASE_URL}}`. No database is
created. Railway creates the bucket and supplies both jobs with generated
reference variables. Credentials are never stored in this repository.

All defaults are configurable. Native Railway cron schedules use UTC and are
edited under each service's **Settings → Cron Schedule**.

| Setting | Template default | Where to change it |
| --- | --- | --- |
| Backup schedule | `0 2 1 * *` (monthly, day 1 at 02:00 UTC) | Backup CRON → Settings |
| Cleanup schedule | `0 3 1 1,7 *` (January/July, day 1 at 03:00 UTC) | Backup Cleanup → Settings |
| `RUN_ON_STARTUP` | `true` (run once and exit) | Backup CRON → Variables |
| `GZIP_CONCURRENCY` | `20` | Backup CRON → Variables |
| `UPLOAD_CONCURRENCY` | `20` | Backup CRON → Variables |
| `PG_VERSION` | `17` (at least your PostgreSQL server version) | Backup CRON → Variables |
| `RETENTION_DAYS` | `183` | Backup Cleanup → Variables |
| `KEEP_MIN` | `1` | Backup Cleanup → Variables |
| `DRY_RUN` | `true` | Backup Cleanup → Variables |

Cleanup starts in preview mode. Review logs, then set `DRY_RUN=false` to enable
deletion. Keep this bucket dedicated to backups: retention applies to all its
objects. Regularly test restoring backups; successful upload alone does not
verify recoverability.

For external S3 storage, update bucket, endpoint, region and credentials on
**both** jobs. The template creates a Railway bucket by default.

## Development

Run `go build`, `go vet` and `go test` from each component directory.
See [backup instructions](backup/README.md) and
[cleanup instructions](cleanup/README.md) for the exact commands and variables.

Build the images from the repository root:

```sh
docker build --build-arg PG_VERSION=17 -t postgres-s3-backup ./backup
docker build -t postgres-s3-cleanup ./cleanup
```

## Upstream synchronization and migration

Backup is synchronized with [brody192/pg_backup](https://github.com/brody192/pg_backup)
through `0b47df8`. Its Go implementation lives under `backup/`; this project adds
OpenTelemetry tracing and configurable S3 path-style addressing. Cleanup is also
implemented in Go and maintained separately under `cleanup/`.

The former Node implementation and Bun cleanup were replaced. Existing services
must use the corresponding root directory. Remove the cleanup's old
`bun run index.ts` start override and use its container entrypoint.

The Go backup requires `RUN_ON_STARTUP=true`. It runs once and exits, with
scheduling controlled by Railway Cron. The old `NODE_VERSION`,
`SINGLE_SHOT_MODE`, `BACKUP_CRON_SCHEDULE`, `BACKUP_OPTIONS`,
`BACKUP_FILE_PREFIX`, `BUCKET_SUBFOLDER`, and `SUPPORT_OBJECT_LOCK` settings
are not supported by Brody's Go implementation.

## Acknowledgements

Thank you to [Brody](https://github.com/brody192) for the original
[pg_backup project](https://github.com/brody192/pg_backup), including its Go
streaming backup implementation. This project builds on that work and adds
backup tracing, a configurable cleanup service, and the combined Railway template.

Thanks also to the maintainers and contributors of the earlier
[Railway PostgreSQL S3 backup template](https://github.com/railwayapp-templates/postgres-s3-backups),
which provided the foundation for this project.

## Marketplace publication

In the Railway template editor, open **Marketplace**, review its PostgreSQL
icon, category and overview, apply any pending changes, then click
**Publish to marketplace**.
See [Railway's guide](https://docs.railway.com/templates/publish-and-share).
