# Postgres S3 Backups with Cleanup

[![Deploy on Railway](https://railway.com/button.svg)](https://railway.com/deploy/postgres-s3-backups-with-cleanup)

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

The image-based template uses two public, prebuilt container images from GitHub
Container Registry (GHCR). Railway does not need a GitHub source connection or
build the Go applications. Both services use the same fixed release version:

| Service | Image (example release) |
| --- | --- |
| Backup CRON | `ghcr.io/maxstue/postgres-s3-backups-cleanup-backup:2026.11.0` |
| Backup Cleanup | `ghcr.io/maxstue/postgres-s3-backups-cleanup-cleanup:2026.11.0` |

These are image sources, not additional services. Variables, cron schedules and
the bucket are still provisioned by the template. GitHub Actions tests and
publishes the images; Railway deployments are independent of repository CI.

| Setting | Template default | Where to change it |
| --- | --- | --- |
| Backup schedule | `0 2 1 * *` (monthly, day 1 at 02:00 UTC) | Backup CRON → Settings |
| Cleanup schedule | `0 3 1 1,7 *` (January/July, day 1 at 03:00 UTC) | Backup Cleanup → Settings |
| `RUN_ON_STARTUP` | `true` | Backup CRON → Variables |
| `GZIP_CONCURRENCY` | `20` | Backup CRON → Variables |
| `UPLOAD_CONCURRENCY` | `20` | Backup CRON → Variables |
| `RETENTION_DAYS` | `183` | Backup Cleanup → Variables |
| `KEEP_MIN` | `1` | Backup Cleanup → Variables |
| `DRY_RUN` | `true` | Backup Cleanup → Variables |

All defaults are configurable. Native Railway cron schedules use UTC. Both
applications run once and exit; Railway starts each scheduled run.

The published backup image contains the PostgreSQL 18 client and supports servers
up to that major version. `PG_VERSION` is a Docker **build argument**, not a runtime
setting: changing a Railway variable cannot change the client in a prebuilt image.
For a different client major version, build a custom image with the argument below.

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

## Versioning and releases

Both applications share one [CalVer](https://calver.org/) version:
`YYYY.M.PATCH`. The year and month use UTC and have no leading zeros; `PATCH`
starts at `0` each month and increases for every further release that month.
For example: `2026.11.0`, `2026.11.1`, then `2026.12.0`.
Git tags have a `v` prefix (`v2026.11.0`); image tags omit it (`2026.11.0`).

CalVer identifies when the application was released; it does not promise SemVer
compatibility. Release notes must explicitly describe breaking changes to
environment variables, backup formats, PostgreSQL client support, or retention
behavior, and provide migration instructions. Review them before upgrading.

### Publish a release

1. Merge the changes into `main` and wait for **CI**: race-enabled Go tests,
   `go vet`, and independent container builds.
2. Create a new tag on the intended commit, for example `v2026.11.0`, and publish
   a non-prerelease GitHub Release with release notes. Drafts, prereleases and
   ordinary pushes do not publish images.
3. **Publish release images** validates the version, repeats CI for the release
   commit, and publishes both images for `linux/amd64` and `linux/arm64` using
   GitHub's built-in `GITHUB_TOKEN`. No registry password needs to be configured.
   The workflow summary records each image digest. Actions are pinned to commits.
4. On first publication, set **both** GHCR packages to **Public** in their package
   settings. New packages may be private even when their repository is public.
   Verify that both images can be pulled without authentication before using them
   in the public Railway template.
5. Update both image sources in the Railway template to the tested release tag,
   then publish the template changes. Existing installations are not migrated
   automatically by editing the template.

Release tags and image versions must never be moved or overwritten. If one image
fails to publish, finish that release before updating the template. For a code
fix, publish a new version. No moving `latest`, year or month tags are published.
The PostgreSQL client is built into the backup image, so client/security updates
also require a new application release.

### Update or roll back an installation

In Railway, change the **Source Image** of each backup service to the desired
version and apply the changes. Keep both services on the same release. Existing
GitHub-backed installations must switch their source from the repository to the
image; merely disabling GitHub autodeploy does not make them image-based.

Keep image auto-updates disabled for deliberate upgrades. The GitHub Upstream
**Check for updates** flow does not apply to image sources. Cron schedules keep
using the installed image until you update it. To roll back, select the previous
known-good image tags (or recorded digests), checking migration instructions first.

## Acknowledgements

Thanks to the maintainers and contributors of [postgres-s3-backups](https://github.com/railwayapp-templates/postgres-s3-backups) for the foundation of this project.

## Marketplace publication

In the Railway template editor, open **Marketplace**, review the listing, apply
any pending changes, then click **Publish to marketplace**.
See [Railway's guide](https://docs.railway.com/templates/publish-and-share).
