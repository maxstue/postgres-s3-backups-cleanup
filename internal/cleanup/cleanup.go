package cleanup

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"slices"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type S3API interface {
	ListObjectsV2(context.Context, *s3.ListObjectsV2Input, ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
	DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
}

type Policy struct {
	RetentionDays, KeepMin int
	DryRun                 bool
}
type Result struct{ Found, Expired, Deleted int }
type object struct {
	key      string
	modified time.Time
}

// Run inventories the entire bucket before deleting, so KEEP_MIN applies across
// all pages and a listing failure cannot produce a partial retention decision.
func Run(ctx context.Context, client S3API, bucket string, policy Policy, now time.Time, log *slog.Logger) (Result, error) {
	var result Result
	if policy.RetentionDays <= 0 || int64(policy.RetentionDays) > math.MaxInt64/int64(24*time.Hour) || policy.KeepMin < 0 {
		return result, errors.New("invalid retention policy")
	}
	var objects []object
	var token *string
	seenTokens := map[string]bool{}
	for {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		page, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: aws.String(bucket), ContinuationToken: token})
		if err != nil {
			return result, fmt.Errorf("list backups: %w", err)
		}
		for _, item := range page.Contents {
			if item.Key != nil && *item.Key != "" && item.LastModified != nil {
				objects = append(objects, object{key: *item.Key, modified: *item.LastModified})
			}
		}
		if !aws.ToBool(page.IsTruncated) {
			break
		}
		token = page.NextContinuationToken
		if token == nil || *token == "" || seenTokens[*token] {
			return result, errors.New("invalid S3 pagination token")
		}
		seenTokens[*token] = true
	}
	slices.SortStableFunc(objects, func(a, b object) int { return b.modified.Compare(a.modified) })
	result.Found = len(objects)
	cutoff := now.Add(-time.Duration(policy.RetentionDays) * 24 * time.Hour)
	for i, item := range objects {
		if i < policy.KeepMin || !item.modified.Before(cutoff) {
			continue
		}
		result.Expired++
		if policy.DryRun {
			log.Info("Would delete backup", slog.String("key", item.key))
			continue
		}
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if _, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(bucket), Key: aws.String(item.key)}); err != nil {
			return result, fmt.Errorf("delete backup: %w", err)
		}
		result.Deleted++
		log.Info("Deleted backup", slog.String("key", item.key))
	}
	log.Info("Backup cleanup complete", slog.Int("found", result.Found), slog.Int("expired", result.Expired),
		slog.Int("deleted", result.Deleted), slog.Bool("dry_run", policy.DryRun), slog.Int("keep_min", policy.KeepMin))
	return result, nil
}
