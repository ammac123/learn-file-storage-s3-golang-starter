package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
)

func (cfg apiConfig) ensureAssetsDir() error {
	if _, err := os.Stat(cfg.assetsRoot); os.IsNotExist(err) {
		return os.Mkdir(cfg.assetsRoot, 0755)
	}
	return nil
}

func getAssetPath(mediaType string) string {
	base := make([]byte, 32)
	_, err := rand.Read(base)
	if err != nil {
		panic("failed to generate random bytes")
	}
	id := base64.RawURLEncoding.EncodeToString(base)

	ext := mediaTypeToExt(mediaType)
	return fmt.Sprintf("%s%s", id, ext)
}

func (cfg apiConfig) getObjectURL(key string) string {
	return fmt.Sprintf("%s/%s", cfg.s3CfDistribution, key)
}

func (cfg apiConfig) getAssetDiskPath(assetPath string) string {
	return filepath.Join(cfg.assetsRoot, assetPath)
}

func (cfg apiConfig) getAssetURL(assetPath string) string {
	return fmt.Sprintf("http://localhost:%s/assets/%s", cfg.port, assetPath)
}

func mediaTypeToExt(mediaType string) string {
	parts := strings.Split(mediaType, "/")
	if len(parts) != 2 {
		return ".bin"
	}
	return "." + parts[1]
}

func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error", "-print_format", "json", "-show_streams",
		filePath,
	)

	var out bytes.Buffer
	cmd.Stdout = &out

	cmd.Run()
	ffprobe := FFProbe{}
	if err := json.Unmarshal(out.Bytes(), &ffprobe); err != nil {
		return "", err
	}
	ratio := calculateRatio(ffprobe.Streams[0].Width, ffprobe.Streams[0].Height)
	return ratio, nil
}

func calculateRatio(w, h int) string {
	const tolerance = 64
	if math.Abs(float64(w*9)-float64(h*16)) <= float64(tolerance) {
		return "16:9"
	} else if math.Abs(float64(h*9)-float64(w*16)) <= float64(tolerance) {
		return "9:16"
	} else {
		return "other"
	}
}

func processVideoForFastStart(filePath string) (string, error) {
	outputFilePath := filePath + ".processing"
	cmd := exec.Command("ffmpeg",
		"-i", filePath,
		"-c", "copy", "-movflags", "faststart",
		"-f", "mp4", outputFilePath,
	)
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return outputFilePath, nil
}

func generateNewPresignedURL(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s3Client)

	if presignClient == nil {
		return "", fmt.Errorf("Sould not set presign client")
	}
	presignedHTTPRequest, err := presignClient.PresignGetObject(
		context.Background(),
		&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		},
		s3.WithPresignExpires(expireTime))
	if err != nil {
		return "", err
	}

	return presignedHTTPRequest.URL, nil
}

func (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) {
	if video.VideoURL == nil {
		return video, nil
	}
	url := *video.VideoURL
	parts := strings.Split(string(url), ",")
	if len(parts) != 2 {
		return video, fmt.Errorf("Incorrect video url for video %v\n", video.ID)
	}

	bucket := parts[0]
	key := parts[1]

	signedURL, err := generateNewPresignedURL(cfg.s3Client, bucket, key, time.Duration(4)*time.Minute)
	if err != nil {
		return video, err
	}
	video.VideoURL = &signedURL
	return video, nil
}
