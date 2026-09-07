//go:build integration

package integration

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/cache"
	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/config"
	"github.com/NeoStackLab/NexaFlow/backend/internal/service"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func TestRedisRoundTrip(t *testing.T) {
	address := strings.TrimSpace(os.Getenv("NEXAFLOW_TEST_REDIS_ADDR"))
	if address == "" {
		t.Skip("NEXAFLOW_TEST_REDIS_ADDR is not set")
	}
	host, portValue, err := net.SplitHostPort(address)
	if err != nil {
		t.Fatalf("parse redis address: %v", err)
	}
	port, err := strconv.Atoi(portValue)
	if err != nil {
		t.Fatalf("parse redis port: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := cache.Open(ctx, config.RedisConfig{Host: host, Port: port, DialTimeout: 3 * time.Second, ReadTimeout: 3 * time.Second, WriteTimeout: 3 * time.Second})
	if err != nil {
		t.Fatalf("open redis: %v", err)
	}
	defer client.Close()
	key := "nexaflow:integration:" + strconv.FormatInt(time.Now().UnixNano(), 10)
	if err := client.Set(ctx, key, "ready", time.Minute).Err(); err != nil {
		t.Fatalf("set redis value: %v", err)
	}
	defer client.Del(context.Background(), key)
	value, err := client.Get(ctx, key).Result()
	if err != nil || value != "ready" {
		t.Fatalf("redis round trip value = %q, error = %v", value, err)
	}
}

func TestS3ObjectStoreRoundTrip(t *testing.T) {
	endpoint := strings.TrimRight(strings.TrimSpace(os.Getenv("NEXAFLOW_TEST_S3_ENDPOINT")), "/")
	if endpoint == "" {
		t.Skip("NEXAFLOW_TEST_S3_ENDPOINT is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	const (
		region    = "us-east-1"
		accessKey = "test"
		secretKey = "test"
	)
	bucket := "nexaflow-integration-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKey, secretKey, "")),
	)
	if err != nil {
		t.Fatalf("load S3 configuration: %v", err)
	}
	admin := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(endpoint)
		options.UsePathStyle = true
	})
	if _, err := admin.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)}); err != nil {
		t.Fatalf("create S3 test bucket: %v", err)
	}
	defer admin.DeleteBucket(context.Background(), &s3.DeleteBucketInput{Bucket: aws.String(bucket)})

	store, err := service.NewObjectStore(ctx, config.StorageConfig{Provider: "s3", Endpoint: endpoint, Region: region, Bucket: bucket, AccessKeyID: accessKey, SecretAccessKey: secretKey})
	if err != nil {
		t.Fatalf("create S3 object store: %v", err)
	}
	payload := []byte("NexaFlow S3 integration contract")
	key := "integration/round-trip.txt"
	if err := store.Put(ctx, key, bytes.NewReader(payload), int64(len(payload)), "text/plain"); err != nil {
		t.Fatalf("put S3 object: %v", err)
	}
	body, err := store.Open(ctx, key)
	if err != nil {
		t.Fatalf("open S3 object: %v", err)
	}
	read, readErr := io.ReadAll(body)
	closeErr := body.Close()
	if readErr != nil || closeErr != nil {
		t.Fatalf("read S3 object: read error = %v, close error = %v", readErr, closeErr)
	}
	if !bytes.Equal(read, payload) {
		t.Fatalf("S3 payload = %q, want %q", read, payload)
	}
	if err := store.Delete(ctx, key); err != nil {
		t.Fatalf("delete S3 object: %v", err)
	}
	if _, err := store.Open(ctx, key); err == nil {
		t.Fatal("deleted S3 object is still readable")
	}
	if store.Provider() != "s3" {
		t.Fatal(fmt.Sprintf("object store provider = %q, want s3", store.Provider()))
	}
}
