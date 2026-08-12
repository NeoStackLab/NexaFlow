package service

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/NeoStackLab/NexaFlow/backend/internal/pkg/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
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
func (s *localObjectStore) Put(_ context.Context, key string, reader io.Reader, size int64, _ string) error {
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
func (s *localObjectStore) Open(_ context.Context, key string) (io.ReadCloser, error) {
	path, err := s.path(key)
	if err != nil {
		return nil, err
	}
	return os.Open(path)
}
func (s *localObjectStore) Delete(_ context.Context, key string) error {
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
func (s *s3ObjectStore) Put(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{Bucket: &s.bucket, Key: &key, Body: reader, ContentLength: aws.Int64(size), ContentType: &contentType})
	return err
}
func (s *s3ObjectStore) Open(ctx context.Context, key string) (io.ReadCloser, error) {
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &key})
	if err != nil {
		return nil, err
	}
	return result.Body, nil
}
func (s *s3ObjectStore) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &s.bucket, Key: &key})
	return err
}
