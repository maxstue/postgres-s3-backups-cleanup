package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/maxstue/postgres-s3-backups-cleanup/internal/cleanup"
	"github.com/maxstue/postgres-s3-backups-cleanup/internal/cleanup/config"
)

func main() { os.Exit(run()) }

func run() int {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		log.Error("Invalid cleanup configuration", "error", err.Error())
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	client := s3.NewFromConfig(aws.Config{Region: cfg.Region,
		Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""))},
		func(o *s3.Options) { o.BaseEndpoint = aws.String(cfg.Endpoint); o.UsePathStyle = cfg.ForcePathStyle })
	if _, err := cleanup.Run(ctx, client, cfg.Bucket, cleanup.Policy{RetentionDays: cfg.RetentionDays, KeepMin: cfg.KeepMin, DryRun: cfg.DryRun}, time.Now(), log); err != nil {
		log.Error("Backup cleanup failed", "error", err.Error())
		return 1
	}
	return 0
}
