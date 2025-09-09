package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
)

func (cfg apiConfig) ensureAssetsDir() error {
	if _, err := os.Stat(cfg.assetsRoot); os.IsNotExist(err) {
		return os.Mkdir(cfg.assetsRoot, 0755)
	}
	return nil
}

func getExtensionFromType(mediaType string) string {
	switch mediaType {
	case "image/png":
		return "png"
	case "image/jpeg":
		return "jpg"
	case "video/mp4":
		return "mp4"
	default:
		return ""
	}
}

func getVideoAspectRatio(filePath string) (string, error) {
	lRatio := 16.0 / 9.0
	pRatio := 9.0 / 16.0

	cmd := exec.Command(
		"ffprobe",
		"-v",
		"error",
		"-print_format",
		"json",
		"-show_streams",
		filePath,
	)

	stdout := bytes.Buffer{}
	cmd.Stdout = &stdout

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("getVideoAspectRatio: %w", err)
	}

	type jsonOutput struct {
		Streams []struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"streams"`
	}
	stdoutJSON := jsonOutput{}

	err = json.Unmarshal(stdout.Bytes(), &stdoutJSON)
	if err != nil {
		return "", fmt.Errorf("getVideoAspectRatio: %w", err)
	}

	if stdoutJSON.Streams[0].Height == 0 {
		return "", fmt.Errorf("invalid video height")
	}

	ratio := float64(stdoutJSON.Streams[0].Width) /
		float64(stdoutJSON.Streams[0].Height)

	if ratio >= lRatio*0.95 && ratio <= lRatio*1.05 {
		return "16:9", nil
	} else if ratio >= pRatio*0.95 && ratio <= pRatio*1.05 {
		return "9:16", nil
	} else {
		return "other", nil
	}
}

func processVideoForFastStart(filepath string) (string, error) {
	outputPath := filepath + ".processing"
	cmd := exec.Command(
		"ffmpeg",
		"-i",
		filepath,
		"-c",
		"copy",
		"-movflags",
		"faststart",
		"-f",
		"mp4",
		outputPath,
	)

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("processVideoForFastStart: %w", err)
	}

	return outputPath, nil
}

func generatePresignedURL(
	s3Client *s3.Client,
	bucket, key string,
	expireTime time.Duration,
) (string, error) {
	presignClient := s3.NewPresignClient(s3Client)

	goi := s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}

	req, err := presignClient.PresignGetObject(
		context.Background(),
		&goi,
		s3.WithPresignExpires(expireTime),
	)
	if err != nil {
		return "", fmt.Errorf("generatePresignedURL: %w", err)
	}

	return req.URL, nil
}

func (cfg *apiConfig) dbVideoToSignedVideo(
	video database.Video,
) (database.Video, error) {
	if video.VideoURL == nil {
		return video, nil
	}

	videoURL := *video.VideoURL

	fmt.Print(videoURL)

	if videoURL == "" {
		return video, nil
	}
	urlParts := strings.Split(videoURL, ",")
	if len(urlParts) < 2 {
		return video, fmt.Errorf("invalid url")
	}

	url, err := generatePresignedURL(
		cfg.s3Client,
		urlParts[0],
		urlParts[1],
		time.Hour,
	)
	if err != nil {
		return video, fmt.Errorf("apiConfig.dbVideoToSignedVideo: %w", err)
	}

	video.VideoURL = &url

	return video, nil
}
