package config

import (
	"errors"
	"github.com/caarlos0/env/v11"
)

type aws struct {
	AccessKeyID     string `env:"AWS_ACCESS_KEY_ID,required,notEmpty"`
	SecretAccessKey string `env:"AWS_SECRET_ACCESS_KEY,required,notEmpty"`
	S3Bucket        string `env:"AWS_S3_BUCKET,required,notEmpty"`
	S3Region        string `env:"AWS_S3_REGION,required,notEmpty"`
	ForcePathStyle  bool   `env:"AWS_S3_FORCE_PATH_STYLE" envDefault:"false"`
	S3Endpoint      string `env:"AWS_S3_ENDPOINT" envDefault:""`
}

type backup struct {
	DatabaseURL       string `env:"BACKUP_DATABASE_URL,required,notEmpty"`
	RunOnStart        bool   `env:"RUN_ON_STARTUP" envDefault:"false"`
	GzipConcurrency   int    `env:"GZIP_CONCURRENCY" envDefault:"20"`
	UploadConcurrency int    `env:"UPLOAD_CONCURRENCY" envDefault:"20"`
}

var (
	AWS    = &aws{}
	Backup = &backup{GzipConcurrency: 20, UploadConcurrency: 20}
)

// Load validates configuration explicitly at application startup. Importing this
// package does not inspect the environment or terminate the process.
func Load() error {
	var awsSettings aws
	var backupSettings backup
	if err := errors.Join(env.Parse(&awsSettings), env.Parse(&backupSettings)); err != nil {
		return err
	}
	*AWS = awsSettings
	*Backup = backupSettings
	return nil
}
