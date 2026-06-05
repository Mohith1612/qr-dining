package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/Mohith1612/qr-dining/internal/config"
)

// R2Client wraps the S3-compatible Cloudflare R2 API.
type R2Client struct {
	bucket     string
	publicBase string
	presigner  *s3.PresignClient
}

// NewR2Client constructs an R2Client from config. Returns nil if R2 is not enabled.
func NewR2Client(cfg config.R2Config) *R2Client {
	if !cfg.Enabled {
		return nil
	}

	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.AccountID)

	s3Client := s3.New(s3.Options{
		Region:      "auto",
		BaseEndpoint: aws.String(endpoint),
		Credentials: aws.NewCredentialsCache(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		),
		UsePathStyle: true,
	})

	return &R2Client{
		bucket:     cfg.Bucket,
		publicBase: strings.TrimRight(cfg.PublicBase, "/"),
		presigner:  s3.NewPresignClient(s3Client),
	}
}

// PresignPut returns a presigned PUT URL that allows direct upload to R2.
// The caller is expected to upload the file with Content-Type set to contentType.
func (r *R2Client) PresignPut(ctx context.Context, key, contentType string, maxBytes int64, ttl time.Duration) (string, error) {
	req, err := r.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(r.bucket),
		Key:           aws.String(key),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(maxBytes),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("presign PUT %s: %w", key, err)
	}
	return req.URL, nil
}

// PublicURL returns the CDN-accessible URL for a given object key.
func (r *R2Client) PublicURL(key string) string {
	return fmt.Sprintf("%s/%s", r.publicBase, key)
}
