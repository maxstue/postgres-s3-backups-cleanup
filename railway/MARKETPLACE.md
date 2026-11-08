# Deploy and Host Postgres S3 Backups with Cleanup on Railway

Back up an existing PostgreSQL database to S3-compatible storage with two
independent, versioned Go applications and a dedicated Railway bucket.

## About Hosting Postgres S3 Backups with Cleanup

- **Backup CRON** streams a PostgreSQL dump through parallel gzip compression to
  S3 multipart uploads. Image:
  `ghcr.io/maxstue/postgres-s3-backups-cleanup-backup:2026.11.0`.
- **Backup Cleanup** previews or deletes expired backups while preserving a
  configurable minimum. Image:
  `ghcr.io/maxstue/postgres-s3-backups-cleanup-cleanup:2026.11.0`.
- **Bucket** provides dedicated S3-compatible object storage, automatically wired
  to both services with generated credential references.

The public images support Linux amd64 and arm64. No GitHub source connection,
repository fork, registry login, or build configuration is required.

## Common Use Cases

- Scheduled logical backups of an existing PostgreSQL database.
- S3-compatible storage with configurable retention and minimum preservation.
- Deliberate upgrades using fixed container versions and backup tracing.

## Dependencies for Postgres S3 Backups with Cleanup Hosting

Deploy into the project containing your PostgreSQL database. Set
`BACKUP_DATABASE_URL` to its connection URL, for example
`${{Postgres.DATABASE_URL}}`. The template does not create a database.
The backup image includes the PostgreSQL 18 client, which supports servers up to
that major version. A runtime `PG_VERSION` variable cannot change a prebuilt image.

| Setting | Default |
| --- | --- |
| Backup schedule | `0 2 1 * *`: day 1 of every month at 02:00 UTC |
| Cleanup schedule | `0 3 1 1,7 *`: January/July, day 1 at 03:00 UTC |
| `RETENTION_DAYS` | `183` days |
| `KEEP_MIN` | Preserve at least `1` newest backup |
| `DRY_RUN` | `true`: preview without deleting |
| `RUN_ON_STARTUP` | `true`: run once and exit when started |
| `GZIP_CONCURRENCY` | `20` compression workers |
| `UPLOAD_CONCURRENCY` | `20` upload workers |

Both applications run once and exit; Railway Cron controls their schedules.
Edit schedules under each service's Settings and runtime settings under Variables.
More workers can increase throughput and memory use.

**Cleanup does not delete backups by default.** Review its logs, then explicitly
set `DRY_RUN=false` to enable deletion. Retention applies to every object in the
bucket; keep it dedicated to backups. Cleanup preserves the newest `KEEP_MIN`
objects even when expired and aborts without deleting if listing fails.

For external S3, configure bucket, endpoint, region and credentials on both
services. Regularly test restoring backups. Logical backups do not provide
continuous point-in-time recovery.

## Versions and updates

The two applications share a CalVer `YYYY.M.PATCH` version, such as `2026.11.0`.
Image tags are fixed release versions. New commits and releases do not deploy
automatically into your project. Review the
[release notes](https://github.com/maxstue/postgres-s3-backups-cleanup/releases)
and change both service image tags when you want to upgrade. Keep image
auto-updates disabled for deliberate upgrades. GitHub Upstream sync is not used
for image sources. To roll back, select the previous known-good versions after
checking migration instructions.

CalVer does not promise SemVer compatibility. Breaking changes and migration
instructions are called out in the release notes. Publishing a template revision
does not automatically migrate existing installations.

## Tracing

Backup supports OpenTelemetry. Enable Railway tracing on **Backup CRON** and leave
automatic instrumentation off. The SDK reads Railway's OTLP configuration and
flushes its traces before exit. Cleanup writes structured logs. Credentials,
connection URLs and backup object keys are not added to span attributes.

## Why Deploy Postgres S3 Backups with Cleanup on Railway?

Railway schedules both jobs and provisions their shared bucket. Public, prebuilt
images keep installations independent of the source repository's deployment
integration; generated variable references wire up storage without credentials
in source control.

## Source and acknowledgements

See the [source repository and release instructions](https://github.com/maxstue/postgres-s3-backups-cleanup).
Thanks to the maintainers and contributors of
[postgres-s3-backups](https://github.com/railwayapp-templates/postgres-s3-backups)
for the foundation of this project.
