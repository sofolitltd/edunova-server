package services

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"edunova-server/config"
)

var (
	presignOnce   sync.Once
	presignClient *s3.PresignClient
)

func presigner() *s3.PresignClient {
	presignOnce.Do(func() {
		c := config.AppConfig
		client := s3.New(s3.Options{
			Region:       c.S3Region,
			BaseEndpoint: aws.String(c.S3Endpoint),
			Credentials:  credentials.NewStaticCredentialsProvider(c.S3AccessKeyID, c.S3SecretAccessKey, ""),
			UsePathStyle: true,
		})
		presignClient = s3.NewPresignClient(client)
	})
	return presignClient
}

func StorageConfigured() bool {
	c := config.AppConfig
	return c.S3Endpoint != "" && c.S3AccessKeyID != "" && c.S3SecretAccessKey != ""
}

func PresignUpload(ctx context.Context, key, contentType string, size int64) (string, error) {
	req, err := presigner().PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(config.AppConfig.S3Bucket),
		Key:           aws.String(key),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(size),
	}, s3.WithPresignExpires(10*time.Minute))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

func PublicURL(key string) string {
	c := config.AppConfig
	if c.S3PublicBaseURL != "" {
		return strings.TrimRight(c.S3PublicBaseURL, "/") + "/" + key
	}
	return strings.TrimRight(c.S3Endpoint, "/") + "/" + c.S3Bucket + "/" + key
}
