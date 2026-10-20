package video

import (
	"bytes"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
)

const (
	Landscape = "landscape"
	Portrait  = "portrait"
	Other     = "other"
)

type FFProbeResult struct {
	Streams []struct {
		Width  int `json:"width,omitempty"`
		Height int `json:"height,omitempty"`
	} `json:"streams"`
}

func GetVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command(
		"ffprobe",
		"-v",
		"error",
		"-print_format",
		"json",
		"-show_streams",
		filePath,
	)

	var out bytes.Buffer
	var errout strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errout

	if err := cmd.Run(); err != nil {
		if errout.Len() != 0 {
			return "", errors.New(errout.String())
		}

		return "", err
	}

	var result FFProbeResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		return "", err
	}
	if result.Streams[0].Width > result.Streams[0].Height {
		return Landscape, nil
	}
	if result.Streams[0].Height > result.Streams[0].Width {
		return Portrait, nil
	}
	return Other, nil
}
