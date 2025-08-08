package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os/exec"
)

const tolerance = 0.1

type VideoMetadata struct {
	Height int `json:"height"`
	Width  int `json:"width"`
}

type FFProbeOutput struct {
	Streams []VideoMetadata
}

func getAspectRatioString(width, height int) (string, error) {
	if width == 0 || height == 0 {
		return "", errors.New("width and height must be greater than zero")
	}

	landscapeTest := (float32(width) / float32(height)) * (9.0 / 16.0)
	portraitTest := (float32(height) / float32(width)) * (9.0 / 16.0)

	if landscapeTest <= 1+tolerance && landscapeTest >= 1-tolerance {
		return "landscape", nil
	} else if portraitTest <= 1+tolerance && portraitTest >= 1-tolerance {
		return "portrait", nil
	} else {
		return "other", nil
	}

}

func getVideoAspectRatio(filePath string) (string, error) {

	var out bytes.Buffer
	var metadata FFProbeOutput

	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-print_format", "json",
		"-show_streams",
		filePath,
	)
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", err
	}

	err = json.Unmarshal(out.Bytes(), &metadata)
	if err != nil {
		return "", err
	}

	return getAspectRatioString(metadata.Streams[0].Width, metadata.Streams[0].Height)
}
