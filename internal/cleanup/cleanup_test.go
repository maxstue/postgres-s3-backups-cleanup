package cleanup

import (
	"context"
	"errors"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"io"
	"log/slog"
	"reflect"
	"testing"
	"time"
)

type fakeS3 struct {
	pages                  []*s3.ListObjectsV2Output
	tokens                 []string
	deleted                []string
	listError, deleteError error
}

func (f *fakeS3) ListObjectsV2(_ context.Context, input *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	f.tokens = append(f.tokens, aws.ToString(input.ContinuationToken))
	if len(f.tokens) > len(f.pages) {
		return nil, f.listError
	}
	return f.pages[len(f.tokens)-1], nil
}
func (f *fakeS3) DeleteObject(_ context.Context, input *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	if f.deleteError != nil {
		return nil, f.deleteError
	}
	f.deleted = append(f.deleted, aws.ToString(input.Key))
	return &s3.DeleteObjectOutput{}, nil
}

var now = time.Date(2026, 11, 7, 0, 0, 0, 0, time.UTC)

func item(key string, age int) types.Object {
	return types.Object{Key: aws.String(key), LastModified: aws.Time(now.Add(-time.Duration(age) * 24 * time.Hour))}
}
func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }
func TestRetentionAcrossPagesAndDryRun(t *testing.T) {
	for _, dryRun := range []bool{false, true} {
		client := &fakeS3{pages: []*s3.ListObjectsV2Output{
			{Contents: []types.Object{item("oldest", 220), item("older", 210)}, IsTruncated: aws.Bool(true), NextContinuationToken: aws.String("next")},
			{Contents: []types.Object{item("newest", 200)}},
		}}
		result, err := Run(context.Background(), client, "backups", Policy{RetentionDays: 183, KeepMin: 1, DryRun: dryRun}, now, quiet())
		if err != nil {
			t.Fatal(err)
		}
		if result.Found != 3 || result.Expired != 2 {
			t.Fatalf("incorrect result: %+v", result)
		}
		if !reflect.DeepEqual(client.tokens, []string{"", "next"}) {
			t.Fatalf("incorrect pagination: %v", client.tokens)
		}
		if dryRun {
			if len(client.deleted) != 0 || result.Deleted != 0 {
				t.Fatal("dry run deleted objects")
			}
		} else if !reflect.DeepEqual(client.deleted, []string{"older", "oldest"}) || result.Deleted != 2 {
			t.Fatalf("incorrect deletions: %v", client.deleted)
		}
	}
}
func TestRetentionBoundaryAndConfigurableMinimum(t *testing.T) {
	for _, test := range []struct {
		keep int
		want []string
	}{{0, []string{"expired"}}, {3, nil}} {
		client := &fakeS3{pages: []*s3.ListObjectsV2Output{{Contents: []types.Object{item("expired", 184), item("boundary", 183), item("recent", 1)}}}}
		_, err := Run(context.Background(), client, "backups", Policy{RetentionDays: 183, KeepMin: test.keep}, now, quiet())
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(client.deleted, test.want) {
			t.Fatalf("minimum %d: deleted %v", test.keep, client.deleted)
		}
	}
}
func TestListingFailureOrBadPaginationNeverDeletes(t *testing.T) {
	for _, token := range []string{"next", ""} {
		listErr := errors.New("listing failed")
		client := &fakeS3{pages: []*s3.ListObjectsV2Output{{Contents: []types.Object{item("old", 200)}, IsTruncated: aws.Bool(true), NextContinuationToken: aws.String(token)}}, listError: listErr}
		_, err := Run(context.Background(), client, "backups", Policy{RetentionDays: 183}, now, quiet())
		if err == nil || len(client.deleted) != 0 {
			t.Fatalf("partial inventory caused deletion: %v", err)
		}
	}
}
func TestDeleteFailureAndCancellation(t *testing.T) {
	expected := errors.New("delete failed")
	client := &fakeS3{pages: []*s3.ListObjectsV2Output{{Contents: []types.Object{item("old", 200)}}}, deleteError: expected}
	if _, err := Run(context.Background(), client, "backups", Policy{RetentionDays: 183}, now, quiet()); !errors.Is(err, expected) {
		t.Fatalf("lost delete error: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client = &fakeS3{}
	if _, err := Run(ctx, client, "backups", Policy{RetentionDays: 183}, now, quiet()); !errors.Is(err, context.Canceled) || len(client.tokens) != 0 {
		t.Fatalf("cancellation ignored: %v", err)
	}
}
func TestInvalidPolicyNeverContactsStorage(t *testing.T) {
	for _, policy := range []Policy{{RetentionDays: 0}, {RetentionDays: 183, KeepMin: -1}, {RetentionDays: 1000000}} {
		client := &fakeS3{}
		if _, err := Run(context.Background(), client, "backups", policy, now, quiet()); err == nil || len(client.tokens) != 0 {
			t.Fatalf("accepted invalid policy: %+v", policy)
		}
	}
}
