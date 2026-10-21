package main

import (
	"context"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/thuyencode/learn-file-storage-s3-golang-starter/internal/database"
)

func generatePresignedURL(
	ctx context.Context,
	s3Client *s3.Client,
	bucket, key string,
	expireTime time.Duration,
) (string, error) {
	req, err := s3.NewPresignClient(s3Client).
		PresignGetObject(ctx, &s3.GetObjectInput{Bucket: &bucket, Key: &key}, s3.WithPresignExpires(expireTime))
	if err != nil {
		return "", err
	}

	return req.URL, nil
}

func (cfg *apiConfig) dbVideoToSignedVideo(
	ctx context.Context,
	video database.Video,
) (database.Video, error) {
	if video.VideoURL == nil {
		return video, nil
	}

	parts := strings.Split(*video.VideoURL, ",")
	if len(parts) < 2 {
		return video, nil
	}

	bucket := parts[0]
	key := parts[1]

	url, err := generatePresignedURL(ctx, cfg.s3Client, bucket, key, time.Minute*10)
	if err != nil {
		return video, err
	}

	video.VideoURL = &url
	return video, nil
}
