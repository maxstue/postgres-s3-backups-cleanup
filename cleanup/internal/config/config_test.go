package config

import (
	"testing"
)

func env(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}
func validEnv() map[string]string {
	return map[string]string{
		"ACCESS_KEY_ID": "fixture", "SECRET_ACCESS_KEY": "fixture", "BUCKET": "backups", "ENDPOINT": "storage.example.com", "RETENTION_DAYS": "183",
	}
}
func TestDefaultsAndOverrides(t *testing.T) {
	values := validEnv()
	cfg, err := Load(env(values))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.DryRun || !cfg.ForcePathStyle || cfg.KeepMin != 1 || cfg.Region != "auto" || cfg.Endpoint != "https://storage.example.com" {
		t.Fatalf("incorrect defaults: %+v", cfg)
	}
	values["DRY_RUN"] = "false"
	values["KEEP_MIN"] = "0"
	values["RETENTION_DAYS"] = "30"
	values["AWS_S3_FORCE_PATH_STYLE"] = "false"
	cfg, err = Load(env(values))
	if err != nil || cfg.DryRun || cfg.ForcePathStyle || cfg.KeepMin != 0 || cfg.RetentionDays != 30 {
		t.Fatalf("overrides failed: %+v %v", cfg, err)
	}
}
func TestInvalidConfigurationFailsClosed(t *testing.T) {
	for _, test := range []struct{ key, value string }{
		{"RETENTION_DAYS", ""}, {"RETENTION_DAYS", "0"}, {"RETENTION_DAYS", "-1"}, {"RETENTION_DAYS", "1.5"},
		{"KEEP_MIN", "-1"}, {"DRY_RUN", "typo"}, {"AWS_S3_FORCE_PATH_STYLE", "typo"},
		{"ENDPOINT", "https://user:secret@storage.example.com"}, {"ENDPOINT", "ftp://storage.example.com"},
		{"ENDPOINT", "https://storage.example.com?secret=token"}, {"SECRET_ACCESS_KEY", ""},
	} {
		t.Run(test.key+"="+test.value, func(t *testing.T) {
			values := validEnv()
			values[test.key] = test.value
			if _, err := Load(env(values)); err == nil {
				t.Fatal("invalid configuration was accepted")
			}
		})
	}
}
