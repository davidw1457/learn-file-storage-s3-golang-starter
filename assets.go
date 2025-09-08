package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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
