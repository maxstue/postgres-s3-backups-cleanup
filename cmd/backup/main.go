package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/maxstue/postgres-s3-backups-cleanup/internal/backup"
	"github.com/maxstue/postgres-s3-backups-cleanup/internal/backup/config"
	"github.com/maxstue/postgres-s3-backups-cleanup/internal/backup/logger"
	"github.com/maxstue/postgres-s3-backups-cleanup/internal/backup/telemetry"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

func main() { os.Exit(run()) }

func run() int {
	if err := config.Load(); err != nil {
		logger.Stderr.Error("Invalid backup configuration", logger.ErrAttr(err))
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if !config.Backup.RunOnStart {
		logger.Stdout.Warn("Skipping database backup, not configured to run on startup")
		return 0
	}
	shutdown, err := telemetry.Setup(ctx)
	if err != nil {
		logger.Stderr.Error("Could not initialize backup tracing")
		return 1
	}
	defer func() {
		flushCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := shutdown(flushCtx); err != nil {
			logger.Stderr.Error("Could not flush backup traces")
		}
	}()
	ctx, span := otel.Tracer("postgres-s3-backups").Start(ctx, "backup.run")
	defer span.End()
	logger.Stdout.Info("Checking access to bucket...")
	if err := backup.CheckBucketAccess(ctx); err != nil {
		span.SetStatus(codes.Error, "")
		logger.Stderr.Error("Error checking bucket access", logger.ErrAttr(err))
		return 1
	}
	logger.Stdout.Info("Access to bucket confirmed")
	logger.Stdout.Info("Starting backup...")
	if err := backup.Run(ctx); err != nil {
		span.SetStatus(codes.Error, "")
		logger.Stderr.Error("Error while running backup.. Exiting..", logger.ErrAttr(err))
		return 1
	}
	return 0
}
