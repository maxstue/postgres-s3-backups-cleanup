package config

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type Config struct {
	AccessKeyID, SecretAccessKey, Bucket, Endpoint, Region string
	RetentionDays, KeepMin                                 int
	DryRun, ForcePathStyle                                 bool
}

func Load(getenv func(string) string) (Config, error) {
	c := Config{AccessKeyID: getenv("ACCESS_KEY_ID"), SecretAccessKey: getenv("SECRET_ACCESS_KEY"),
		Bucket: getenv("BUCKET"), Region: getenv("REGION")}
	for _, entry := range []struct{ name, value string }{
		{"ACCESS_KEY_ID", c.AccessKeyID}, {"SECRET_ACCESS_KEY", c.SecretAccessKey}, {"BUCKET", c.Bucket}, {"ENDPOINT", getenv("ENDPOINT")},
	} {
		if entry.value == "" {
			return c, fmt.Errorf("missing variable %s", entry.name)
		}
	}
	endpoint := strings.TrimSpace(getenv("ENDPOINT"))
	if !strings.Contains(endpoint, "://") {
		endpoint = "https://" + endpoint
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Hostname() == "" || strings.HasSuffix(u.Host, ":") ||
		(u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return c, errors.New("ENDPOINT must be an HTTP(S) URL or hostname without credentials, query or fragment")
	}
	c.Endpoint = u.String()
	if c.Region == "" {
		c.Region = "auto"
	}
	c.RetentionDays, err = strconv.Atoi(getenv("RETENTION_DAYS"))
	if err != nil || c.RetentionDays <= 0 {
		return c, errors.New("RETENTION_DAYS must be a positive integer")
	}
	c.KeepMin = 1
	if raw := getenv("KEEP_MIN"); raw != "" {
		c.KeepMin, err = strconv.Atoi(raw)
		if err != nil || c.KeepMin < 0 {
			return c, errors.New("KEEP_MIN must be a non-negative integer")
		}
	}
	c.DryRun, err = parseBool(getenv("DRY_RUN"), true)
	if err != nil {
		return c, errors.New("DRY_RUN must be a boolean")
	}
	c.ForcePathStyle, err = parseBool(getenv("AWS_S3_FORCE_PATH_STYLE"), true)
	if err != nil {
		return c, errors.New("AWS_S3_FORCE_PATH_STYLE must be a boolean")
	}
	return c, nil
}

func parseBool(raw string, fallback bool) (bool, error) {
	if raw == "" {
		return fallback, nil
	}
	return strconv.ParseBool(raw)
}
