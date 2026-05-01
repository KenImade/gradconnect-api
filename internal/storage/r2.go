package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

// R2Config holds the credentials and bucket info for connecting to a
// Cloudflare R2 bucket via its S3-compatible API.
type R2Config struct {
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
	Bucket          string
	PublicURL       string // e.g. https://pub-abc123.r2.dev
	Endpoint        string // e.g. https://<account-id>.r2.cloudflarestorage.com
}

// R2Storage implements Storage against Cloudflare R2 using the AWS SDK
// pointed at R2's S3-compatible endpoint. R2's API is a strict subset
// of S3's, so anything that uses PutObject/GetObject/DeleteObject works.
type R2Storage struct {
	cfg    R2Config
	client *s3.Client
}

// NewR2Storage constructs a client. Returns an error if the endpoint
// is unreachable or credentials are rejected at first use — initial
// connection is lazy, so most config errors surface on the first Upload.
func NewR2Storage(ctx context.Context, cfg R2Config) (*R2Storage, error) {
	if cfg.Bucket == "" || cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" {
		return nil, errors.New("r2: missing required config (bucket, access key, secret key)")
	}
	if cfg.Endpoint == "" {
		if cfg.AccountID == "" {
			return nil, errors.New("r2: must provide either Endpoint or AccountID")
		}
		cfg.Endpoint = fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.AccountID)
	}

	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("auto"), // R2 ignores region but the SDK requires one
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID, cfg.SecretAccessKey, "",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("r2: loading aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		o.UsePathStyle = true // R2 prefers path-style addressing
	})

	return &R2Storage{cfg: cfg, client: client}, nil
}

func (r *R2Storage) Upload(ctx context.Context, key, contentType string, body io.Reader) (string, error) {
	_, err := r.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(r.cfg.Bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("r2: upload %q: %w", key, err)
	}
	return r.PublicURL(key), nil
}

func (r *R2Storage) Download(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := r.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(r.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("r2: download %q: %w", key, err)
	}
	return out.Body, nil
}

func (r *R2Storage) Delete(ctx context.Context, key string) error {
	_, err := r.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(r.cfg.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		// R2 returns 404 for missing keys; treat as idempotent success.
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchKey" {
			return nil
		}
		return fmt.Errorf("r2: delete %q: %w", key, err)
	}
	return nil
}

func (r *R2Storage) PublicURL(key string) string {
	base := strings.TrimRight(r.cfg.PublicURL, "/")
	return fmt.Sprintf("%s/%s", base, key)
}
