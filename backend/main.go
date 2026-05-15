package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
)

func compressVideo(inputPath, outputPath string) error {
	if !fileExists(inputPath) {
		return fmt.Errorf("input file does not exist: %s", inputPath)
	}

	cmd := exec.Command(
		"ffmpeg",
		"-i", inputPath,
		"-vcodec", "libx264",
		"-crf", "28",
		"-c:a", "aac",
		outputPath,
	)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to compress video: %w", err)
	}
	return nil
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

func main() {
	err := compressVideo("./input/distributed-server_ghx1ne.mp4", "./output/compressed2.mp4")

	if err != nil {
		log.Fatalf("Failed to compress video with ffmpeg my nibba: %s", err)
	}
}
