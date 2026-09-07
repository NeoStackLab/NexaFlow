package service

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
)

func NewObjectStore(ctx context.Context, cfg config.StorageConfig) (ObjectStore, error) {
	if cfg.Provider == "local" {
		root, err := filepath.Abs(cfg.LocalPath)
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(root, 0o750); err != nil {
			return nil, err
		}
		return &localObjectStore{root: root}, nil
	}
	if cfg.Bucket == "" || cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" {
		return nil, fmt.Errorf("storage %s requires bucket and credentials", cfg.Provider)
	}
	options := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(cfg.Region), awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""))}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		if cfg.Endpoint != "" {
			options.BaseEndpoint = aws.String(strings.TrimRight(cfg.Endpoint, "/"))
			options.UsePathStyle = cfg.Provider == "s3"
		}
	})
	return &s3ObjectStore{client: client, bucket: cfg.Bucket, provider: cfg.Provider}, nil
}

type localObjectStore struct{ root string }

func (s *localObjectStore) Provider() string { return "local" }
func (s *localObjectStore) path(key string) (string, error) {
	path := filepath.Join(s.root, filepath.FromSlash(key))
	relative, err := filepath.Rel(s.root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid storage key")
	}
	return path, nil
}
func (s *localObjectStore) Put(ctx context.Context, key string, reader io.Reader, size int64, _ string) (err error) {
	ctx, done := observeStorage(ctx, "local", "put")
	defer func() { done(err) }()
	path, err := s.path(key)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	written, copyErr := io.Copy(file, reader)
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil || written != size {
		_ = os.Remove(path)
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		return fmt.Errorf("uploaded size does not match declared size")
	}
	return nil
}
func (s *localObjectStore) Open(ctx context.Context, key string) (result io.ReadCloser, err error) {
	ctx, done := observeStorage(ctx, "local", "open")
	defer func() { done(err) }()
	path, err := s.path(key)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}
func (s *localObjectStore) Delete(ctx context.Context, key string) (err error) {
	ctx, done := observeStorage(ctx, "local", "delete")
	defer func() { done(err) }()
	path, err := s.path(key)
	if err != nil {
		return err
	}
	if err = os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

type s3ObjectStore struct {
	client   *s3.Client
	bucket   string
	provider string
}

func (s *s3ObjectStore) Provider() string { return s.provider }
func (s *s3ObjectStore) Put(ctx context.Context, key string, reader io.Reader, size int64, contentType string) (err error) {
	ctx, done := observeStorage(ctx, s.provider, "put")
	defer func() { done(err) }()
	_, err = s.client.PutObject(ctx, &s3.PutObjectInput{Bucket: &s.bucket, Key: &key, Body: reader, ContentLength: aws.Int64(size), ContentType: &contentType})
	return err
}
func (s *s3ObjectStore) Open(ctx context.Context, key string) (result io.ReadCloser, err error) {
	ctx, done := observeStorage(ctx, s.provider, "open")
	defer func() { done(err) }()
	response, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &key})
	if err != nil {
		return nil, err
	}
	return response.Body, nil
}
func (s *s3ObjectStore) Delete(ctx context.Context, key string) (err error) {
	ctx, done := observeStorage(ctx, s.provider, "delete")
	defer func() { done(err) }()
	_, err = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &s.bucket, Key: &key})
	return err
}

func observeStorage(ctx context.Context, provider, operation string) (context.Context, func(error)) {
	storageMetrics.once.Do(func() {
		storageMetrics.duration, _ = otel.Meter("nexaflow/object-store").Float64Histogram("nexaflow.object_store.operation.duration", metric.WithUnit("s"))
	})
	started := time.Now()
	ctx, span := otel.Tracer("nexaflow/object-store").Start(ctx, "object_store."+operation)
	span.SetAttributes(attribute.String("storage.provider", provider), attribute.String("storage.operation", operation))
	return ctx, func(err error) {
		storageMetrics.duration.Record(ctx, time.Since(started).Seconds(), metric.WithAttributes(attribute.String("storage.provider", provider), attribute.String("storage.operation", operation)))
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}
}

var storageMetrics struct {
	once     sync.Once
	duration metric.Float64Histogram
}
