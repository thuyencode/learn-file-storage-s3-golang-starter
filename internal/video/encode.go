package video

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
)

var suffix = ".processing"

func ProcessVideoForFastStart(filePath string) (string, error) {
	outputFilePath := filePath + suffix

	cmd := exec.Command(
		"ffmpeg",
		"-i",
		filePath,
		"-c",
		"copy",
		"-movflags",
		"faststart",
		"-f",
		"mp4",
		outputFilePath,
	)

	out := new(bytes.Buffer)
	errout := new(strings.Builder)
	cmd.Stdout = out
	cmd.Stderr = errout

	if err := cmd.Run(); err != nil {
		if errout.Len() != 0 {
			return "", errors.New(errout.String())
		}
		return "", err
	}

	return outputFilePath, nil
}
