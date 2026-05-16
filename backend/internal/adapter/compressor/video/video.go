package video

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/umohsamuel/elcompresso/internal/domain/compress"
)

type VideoCompressor struct {
}

func NewVideoCompressor() compress.Interface {
	return &VideoCompressor{}
}

func (v *VideoCompressor) Compress(req compress.CompressionRequest) (*compress.CompressionResult, error) {
	ext := filepath.Ext(req.FileName)

	inputFile, err := os.CreateTemp("", "input-*"+ext)
	if err != nil {
		return nil, err
	}
	defer os.Remove(inputFile.Name())

	if _, err := io.Copy(inputFile, req.Input); err != nil {
		return nil, err
	}
	inputFile.Close()

	outputFile, err := os.CreateTemp("", "output-*"+ext)
	if err != nil {
		return nil, err
	}
	defer os.Remove(outputFile.Name())
	outputFile.Close()

	args := v.ffmpegArgs(inputFile.Name(), outputFile.Name(), ext)
	cmd := exec.Command(
		"ffmpeg",
		args...,
	)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg failed: %w", err)
	}

	compressed, err := os.Open(outputFile.Name())
	if err != nil {
		return nil, err
	}

	info, _ := compressed.Stat()

	return &compress.CompressionResult{
		Output:         compressed,
		CompressedSize: info.Size(),
	}, nil
}

func (v *VideoCompressor) Supports(fileType compress.FileType, extension string) bool {
	if fileType != compress.FileTypeVideo {
		return false
	}

	supported := map[string]bool{
		".mp4":  true,
		".mkv":  true,
		".avi":  true,
		".mov":  true,
		".webm": true,
		".flv":  true,
	}

	return supported[extension]
}

func (v *VideoCompressor) ffmpegArgs(input, output, ext string) []string {
	switch ext {
	case ".webm":
		return []string{"-y", "-i", input, "-c:v", "libvpx-vp9", "-crf", "30", "-b:v", "0", "-c:a", "libopus", output}
	case ".flv":
		return []string{"-y", "-i", input, "-c:v", "flv1", "-c:a", "mp3", output}
	default:
		return []string{"-y", "-i", input, "-vcodec", "libx264", "-crf", "28", "-c:a", "aac", output}
	}
}
