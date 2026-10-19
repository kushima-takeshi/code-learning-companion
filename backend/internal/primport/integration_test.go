package primport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/kushima-takeshi/code-learning-companion/backend/internal/storage"
)

func TestImportIntegration(t *testing.T) {
	if os.Getenv("PRIMPORT_INTEGRATION_TEST") != "1" {
		t.Skip("PR import integration test is disabled")
	}
	requireEnv := func(name string) string {
		value := os.Getenv(name)
		if value == "" {
			t.Fatalf("%s is required", name)
		}
		return value
	}
	databaseURL := requireEnv("PRIMPORT_TEST_DATABASE_URL")
	endpoint := requireEnv("PRIMPORT_TEST_S3_ENDPOINT")
	accessKey := requireEnv("PRIMPORT_TEST_S3_ACCESS_KEY")
	secretKey := requireEnv("PRIMPORT_TEST_S3_SECRET_KEY")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cfg, err := pgx.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	admin := stdlib.OpenDB(*cfg)
	t.Cleanup(func() { admin.Close() })
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	schema := "primport_test_" + suffix
	if _, err := admin.ExecContext(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if _, err := admin.ExecContext(cleanupCtx, `DROP SCHEMA "`+schema+`" CASCADE`); err != nil {
			t.Errorf("drop test schema: %v", err)
		}
	})
	cfg.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*cfg)
	t.Cleanup(func() { db.Close() })
	for _, name := range []string{"000002_auth.up.sql", "000003_pr_import.up.sql"} {
		body, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := db.ExecContext(ctx, string(body)); err != nil {
			t.Fatal(err)
		}
	}
	bucket := "primport-test-" + suffix
	client := s3.NewFromConfig(aws.Config{
		Region: "us-east-1", Credentials: credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
	}, func(o *s3.Options) { o.BaseEndpoint = aws.String(endpoint); o.UsePathStyle = true })
	store := storage.New(endpoint, accessKey, secretKey, bucket)
	if err := store.EnsureBucket(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		pages := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{Bucket: aws.String(bucket)})
		for pages.HasMorePages() {
			page, err := pages.NextPage(cleanupCtx)
			if err != nil {
				t.Errorf("list test objects: %v", err)
				return
			}
			for _, object := range page.Contents {
				if err := store.Delete(cleanupCtx, aws.ToString(object.Key)); err != nil {
					t.Errorf("delete test object: %v", err)
				}
			}
		}
		if _, err := client.DeleteBucket(cleanupCtx, &s3.DeleteBucketInput{Bucket: aws.String(bucket)}); err != nil {
			t.Errorf("delete test bucket: %v", err)
		}
	})
	_, fetcher, _, _ := testService(t)
	service := &Service{db: db, github: fetcher, store: store}
	owner, other := uuid.NewString(), uuid.NewString()
	for i, id := range []string{owner, other} {
		if _, err := db.ExecContext(ctx, "INSERT INTO users (id, github_user_id, github_login) VALUES ($1, $2, $3)", id, i+1, fmt.Sprintf("test-%d", i)); err != nil {
			t.Fatal(err)
		}
	}
	importPR := func(userID string, file *UploadFile) (*ImportResult, error) {
		input := testInput()
		input.UserID = userID
		input.File = file
		return service.Import(ctx, input)
	}
	countObjects := func() int {
		t.Helper()
		count := 0
		pages := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{Bucket: aws.String(bucket)})
		for pages.HasMorePages() {
			page, err := pages.NextPage(ctx)
			if err != nil {
				t.Fatal(err)
			}
			count += len(page.Contents)
		}
		return count
	}
	countRows := func(table string) int {
		t.Helper()
		var count int
		if err := db.QueryRowContext(ctx, "SELECT count(*) FROM "+table).Scan(&count); err != nil {
			t.Fatal(err)
		}
		return count
	}
	t.Run("save_list_get_and_owner_isolation", func(t *testing.T) {
		first, err := importPR(owner, conversationFile("# Integration conversation"))
		if err != nil {
			t.Fatal(err)
		}
		second, err := importPR(owner, nil)
		if err != nil {
			t.Fatal(err)
		}
		if first.SnapshotID == second.SnapshotID {
			t.Fatal("duplicate snapshot IDs")
		}
		if _, err := db.ExecContext(ctx, "UPDATE pr_snapshots SET created_at=$1 WHERE id=$2", time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), first.SnapshotID); err != nil {
			t.Fatal(err)
		}
		list, err := service.List(ctx, owner)
		if err != nil {
			t.Fatal(err)
		}
		if len(list) != 2 || list[0].ID != second.SnapshotID || list[1].ID != first.SnapshotID {
			t.Fatalf("wrong list order: %+v", list)
		}
		others, err := service.List(ctx, other)
		if err != nil || len(others) != 0 {
			t.Fatalf("other user's list=%+v err=%v", others, err)
		}
		got, err := service.Get(ctx, owner, first.SnapshotID)
		if err != nil || got.ID != first.SnapshotID {
			t.Fatalf("get=%+v err=%v", got, err)
		}
		for _, id := range []string{first.SnapshotID, uuid.NewString()} {
			got, err := service.Get(ctx, other, id)
			if got != nil || !errors.Is(err, ErrSnapshotNotFound) {
				t.Fatalf("unauthorized/missing get=%+v err=%v", got, err)
			}
		}
		var conversationKey, bodyKey string
		var linkedUser string
		if err := db.QueryRowContext(ctx, "SELECT object_key, user_id FROM conversation_uploads WHERE pr_snapshot_id=$1", first.SnapshotID).Scan(&conversationKey, &linkedUser); err != nil {
			t.Fatal(err)
		}
		if linkedUser != owner {
			t.Fatal("wrong conversation owner")
		}
		if err := db.QueryRowContext(ctx, "SELECT body_key FROM pr_snapshots WHERE id=$1", first.SnapshotID).Scan(&bodyKey); err != nil {
			t.Fatal(err)
		}
		for key, want := range map[string]string{bodyKey: "PR body", conversationKey: "# Integration conversation"} {
			object, err := client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
			if err != nil {
				t.Fatal(err)
			}
			body, readErr := io.ReadAll(object.Body)
			object.Body.Close()
			if readErr != nil || string(body) != want {
				t.Fatalf("saved body=%q err=%v", body, readErr)
			}
		}
		if countRows("pr_snapshots") != 2 || countRows("conversation_uploads") != 1 || countObjects() != 15 {
			t.Fatal("unexpected saved counts")
		}
	})
	t.Run("invalid_conversation_still_saves_pr", func(t *testing.T) {
		snapshots, conversations, objects := countRows("pr_snapshots"), countRows("conversation_uploads"), countObjects()
		result, err := importPR(owner, &UploadFile{Filename: "bad.txt", Reader: strings.NewReader("bad")})
		if err != nil || result.Warning != "invalid_file_extension" {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		if countRows("pr_snapshots") != snapshots+1 || countRows("conversation_uploads") != conversations || countObjects() != objects+7 {
			t.Fatal("invalid conversation changed unexpected data")
		}
	})
	t.Run("foreign_key_failure_cleans_objects", func(t *testing.T) {
		snapshots, conversations, objects := countRows("pr_snapshots"), countRows("conversation_uploads"), countObjects()
		result, err := importPR(uuid.NewString(), conversationFile("hello"))
		if result != nil || err == nil {
			t.Fatal("expected foreign key error")
		}
		if countRows("pr_snapshots") != snapshots || countRows("conversation_uploads") != conversations || countObjects() != objects {
			t.Fatal("failed import left data")
		}
	})
	t.Run("conversation_insert_failure_rolls_back_snapshot", func(t *testing.T) {
		if _, err := db.ExecContext(ctx, "ALTER TABLE conversation_uploads ADD CONSTRAINT reject_test_filename CHECK (filename <> 'reject.md')"); err != nil {
			t.Fatal(err)
		}
		snapshots, conversations, objects := countRows("pr_snapshots"), countRows("conversation_uploads"), countObjects()
		result, err := importPR(owner, &UploadFile{Filename: "reject.md", Reader: strings.NewReader("valid markdown")})
		if result != nil || err == nil {
			t.Fatal("expected conversation insert failure")
		}
		if countRows("pr_snapshots") != snapshots || countRows("conversation_uploads") != conversations || countObjects() != objects {
			t.Fatal("failed conversation import left data")
		}
	})
}
