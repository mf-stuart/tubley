package main

import (
	"bytes"
	"fmt"
	"os/exec"
)

func processVideoForFastStart(filepath string) (string, error) {
	processingPath := fmt.Sprintf("%s.%s", filepath, "processing")

	var out bytes.Buffer
	cmd := exec.Command("ffmpeg",
		"-i", filepath,
		"-c", "copy",
		"-movflags", "faststart",
		"-f", "mp4",
		processingPath,
	)
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", err
	}

	return processingPath, nil
}
