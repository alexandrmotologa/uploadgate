package storage

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"
)

// S3Provider implements Provider for AWS S3, Cloudflare R2, and MinIO.
type S3Provider struct {
	name          string
	bucket        string
	region        string
	publicBaseURL string
	client        *s3.Client
	presignClient *s3.PresignClient
}

// S3Options defines the configuration parameters for S3Provider.
type S3Options struct {
	Name            string
	Endpoint        string
	Bucket          string
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	PublicBaseURL   string
	UsePathStyle    bool
}

// NewS3Provider instantiates a new S3/R2/MinIO storage provider.
func NewS3Provider(opts S3Options) (*S3Provider, error) {
	if opts.Bucket == "" {
		return nil, fmt.Errorf("s3 bucket name is required")
	}
	if opts.Region == "" {
		opts.Region = "us-east-1"
	}

	creds := credentials.NewStaticCredentialsProvider(opts.AccessKeyID, opts.SecretAccessKey, "")

	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		if opts.Endpoint != "" {
			return aws.Endpoint{
				URL:               opts.Endpoint,
				SigningRegion:     opts.Region,
				HostnameImmutable: opts.UsePathStyle,
			}, nil
		}
		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	})

	cfg := aws.Config{
		Region:                      opts.Region,
		Credentials:                 creds,
		EndpointResolverWithOptions: customResolver,
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = opts.UsePathStyle
	})

	presignClient := s3.NewPresignClient(client)

	baseURL := strings.TrimRight(opts.PublicBaseURL, "/")
	if baseURL == "" {
		if opts.Endpoint != "" {
			if opts.UsePathStyle {
				baseURL = fmt.Sprintf("%s/%s", strings.TrimRight(opts.Endpoint, "/"), opts.Bucket)
			} else {
				baseURL = opts.Endpoint
			}
		} else {
			baseURL = fmt.Sprintf("https://%s.s3.%s.amazonaws.com", opts.Bucket, opts.Region)
		}
	}

	return &S3Provider{
		name:          opts.Name,
		bucket:        opts.Bucket,
		region:        opts.Region,
		publicBaseURL: baseURL,
		client:        client,
		presignClient: presignClient,
	}, nil
}

func (s *S3Provider) Name() string {
	return s.name
}

func (s *S3Provider) Type() string {
	return "s3"
}

// PresignPut generates an AWS SigV4 presigned PUT URL.
func (s *S3Provider) PresignPut(ctx context.Context, key string, mimeType string, sizeBytes int64, expiry time.Duration) (*PresignResult, error) {
	uploadID := "upl_" + strings.ReplaceAll(uuid.New().String(), "-", "")

	input := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}
	if mimeType != "" {
		input.ContentType = aws.String(mimeType)
	}
	if sizeBytes > 0 {
		input.ContentLength = aws.Int64(sizeBytes)
	}

	presignedReq, err := s.presignClient.PresignPutObject(ctx, input, s3.WithPresignExpires(expiry))
	if err != nil {
		return nil, fmt.Errorf("failed to presign PUT request: %w", err)
	}

	headers := make(map[string]string)
	if mimeType != "" {
		headers["Content-Type"] = mimeType
	}
	for k, v := range presignedReq.SignedHeader {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	return &PresignResult{
		UploadID:   uploadID,
		URL:        presignedReq.URL,
		Method:     presignedReq.Method,
		Headers:    headers,
		StorageKey: key,
		ExpiresAt:  time.Now().Add(expiry),
	}, nil
}

// InitiateMultipart starts a multipart upload session in S3.
func (s *S3Provider) InitiateMultipart(ctx context.Context, key string, mimeType string) (string, error) {
	input := &s3.CreateMultipartUploadInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}
	if mimeType != "" {
		input.ContentType = aws.String(mimeType)
	}

	resp, err := s.client.CreateMultipartUpload(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to initiate multipart upload: %w", err)
	}

	return *resp.UploadId, nil
}

// PresignPart generates a presigned URL for an individual chunk in a multipart upload.
func (s *S3Provider) PresignPart(ctx context.Context, key string, multipartUploadID string, partNumber int32, expiry time.Duration) (string, error) {
	input := &s3.UploadPartInput{
		Bucket:     aws.String(s.bucket),
		Key:        aws.String(key),
		UploadId:   aws.String(multipartUploadID),
		PartNumber: aws.Int32(partNumber),
	}

	presignedReq, err := s.presignClient.PresignUploadPart(ctx, input, s3.WithPresignExpires(expiry))
	if err != nil {
		return "", fmt.Errorf("failed to presign upload part %d: %w", partNumber, err)
	}

	return presignedReq.URL, nil
}

// CompleteMultipart finalizes an S3 multipart upload with the list of chunk ETags.
func (s *S3Provider) CompleteMultipart(ctx context.Context, key string, multipartUploadID string, parts []CompletedPart) error {
	var s3Parts []types.CompletedPart
	for _, p := range parts {
		s3Parts = append(s3Parts, types.CompletedPart{
			PartNumber: aws.Int32(p.PartNumber),
			ETag:       aws.String(p.ETag),
		})
	}

	input := &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(s.bucket),
		Key:      aws.String(key),
		UploadId: aws.String(multipartUploadID),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: s3Parts,
		},
	}

	_, err := s.client.CompleteMultipartUpload(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to complete multipart upload: %w", err)
	}

	return nil
}

// GetObject streams the uploaded object bytes from S3.
func (s *S3Provider) GetObject(ctx context.Context, key string) (io.ReadCloser, int64, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}

	resp, err := s.client.GetObject(ctx, input)
	if err != nil {
		return nil, 0, fmt.Errorf("s3 get object failed: %w", err)
	}

	var size int64 = 0
	if resp.ContentLength != nil {
		size = *resp.ContentLength
	}

	return resp.Body, size, nil
}

// PutObject writes bytes directly to S3 (e.g. for generated thumbnails).
func (s *S3Provider) PutObject(ctx context.Context, key string, body io.Reader, sizeBytes int64, contentType string) error {
	input := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   body,
	}
	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}
	if sizeBytes > 0 {
		input.ContentLength = aws.Int64(sizeBytes)
	}

	_, err := s.client.PutObject(ctx, input)
	if err != nil {
		return fmt.Errorf("s3 put object failed: %w", err)
	}

	return nil
}

// DeleteObject deletes an object from S3.
func (s *S3Provider) DeleteObject(ctx context.Context, key string) error {
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}

	_, err := s.client.DeleteObject(ctx, input)
	return err
}

// PublicURL returns the accessible CDN / HTTP URL.
func (s *S3Provider) PublicURL(key string) string {
	cleanKey := strings.TrimPrefix(key, "/")
	return fmt.Sprintf("%s/%s", s.publicBaseURL, cleanKey)
}

// CheckHealth verifies connectivity to the S3 bucket.
func (s *S3Provider) CheckHealth(ctx context.Context) error {
	input := &s3.HeadBucketInput{
		Bucket: aws.String(s.bucket),
	}
	_, err := s.client.HeadBucket(ctx, input)
	if err != nil {
		return fmt.Errorf("s3 head bucket failed for %q: %w", s.bucket, err)
	}
	return nil
}
