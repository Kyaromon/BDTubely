package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
)

type ffprobeOutput struct {
	Streams []struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"streams"`
}

func abs(x float64) float64 {
    if x < 0 {
        return -x
    }
    return x
}

func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

if err := cmd.Run(); err != nil {
	return "", fmt.Errorf("failed to run ffprobe: %w", err)
}

var output ffprobeOutput
if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
	return "", fmt.Errorf("failed to unmarshal ffprobe output: %w", err)
}

if len(output.Streams) == 0 {
	return "", fmt.Errorf("no video streams found")
}

width := output.Streams[0].Width
height := output.Streams[0].Height

if width == 0 || height == 0 {
    return "", fmt.Errorf("invalid width or height")
}

w := float64(width)
h := float64(height)
ratio := w / h

const eps = 0.03

if abs(ratio-16.0/9.0) <= eps {
    return "16:9", nil
} else if abs(ratio-9.0/16.0) <= eps {
    return "9:16", nil
} else if ratio > 1.0+eps {
    return "16:9", nil
} else if ratio < 1.0-eps {
    return "9:16", nil
} else {
    return "other", nil
}
}