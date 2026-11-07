package config

import "testing"

func TestLoadValidatesWithoutReplacingConfigurationOnFailure(t *testing.T) {
	previousAWS, previousBackup := *AWS, *Backup
	t.Cleanup(func() { *AWS = previousAWS; *Backup = previousBackup })
	for key, value := range map[string]string{
		"AWS_ACCESS_KEY_ID": "fixture", "AWS_SECRET_ACCESS_KEY": "fixture", "AWS_S3_BUCKET": "fixture",
		"AWS_S3_REGION": "auto", "BACKUP_DATABASE_URL": "postgresql://fixture@localhost/fixture",
		"RUN_ON_STARTUP": "true", "GZIP_CONCURRENCY": "2", "UPLOAD_CONCURRENCY": "3",
	} {
		t.Setenv(key, value)
	}
	if err := Load(); err != nil {
		t.Fatal(err)
	}
	if !Backup.RunOnStart || Backup.GzipConcurrency != 2 || Backup.UploadConcurrency != 3 {
		t.Fatal("configuration not loaded")
	}
	loaded := *Backup
	t.Setenv("BACKUP_DATABASE_URL", "")
	if err := Load(); err == nil {
		t.Fatal("missing database URL was accepted")
	}
	if *Backup != loaded {
		t.Fatal("failed load replaced valid configuration")
	}
}
