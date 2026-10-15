package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

func TestMinIOPutDelete(t *testing.T) {
	if os.Getenv("MINIO_INTEGRATION_TEST") != "1" {
		t.Skip("MinIO integration test is disabled")
	}

	store := New(
		os.Getenv("S3_ENDPOINT"),
		os.Getenv("S3_ACCESS_KEY"),
		os.Getenv("S3_SECRET_KEY"),
		os.Getenv("S3_BUCKET"),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := store.EnsureBucket(ctx); err != nil {
		t.Fatal(err)
	}

	key := fmt.Sprintf(
		"integration-test/%d/%s",
		time.Now().UnixNano(),
		SanitizePath("src/../hello.txt"),
	)

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := store.Delete(cleanupCtx, key); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})

	const want = "Hello, MinIO!\n"

	if err := store.Put(
		ctx, key, strings.NewReader(want), "text/plain; charset=utf-8",
	); err != nil {
		t.Fatal(err)
	}

	result, err := store.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(store.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		t.Fatal(err)
	}

	body, readErr := io.ReadAll(result.Body)
	result.Body.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(body) != want {
		t.Fatalf("body = %q, want %q", string(body), want)
	}
	t.Log("保存した内容が一致しました")

	if err := store.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}

	_, err = store.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(store.bucket),
		Key:    aws.String(key),
	})

	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) || apiErr.ErrorCode() != "NotFound" {
		t.Fatalf("削除後は NotFound を期待しました: %v", err)
	}
	t.Log("削除後にオブジェクトが存在しないことを確認しました")
}
