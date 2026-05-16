package audio

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/umohsamuel/elcompresso/internal/domain/compress"
)

type AudioCompressor struct{}

func NewAudioCompressor() compress.Interface {
	return &AudioCompressor{}
}

func (a *AudioCompressor) Compress(req compress.CompressionRequest) (*compress.CompressionResult, error) {
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

	args := a.ffmpegArgs(inputFile.Name(), outputFile.Name(), ext)
	cmd := exec.Command("ffmpeg", args...)
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

func (a *AudioCompressor) Supports(fileType compress.FileType, extension string) bool {
	if fileType != compress.FileTypeAudio {
		return false
	}
	supported := map[string]bool{
		".mp3":  true,
		".wav":  true,
		".flac": true,
		".aac":  true,
		".ogg":  true,
		".m4a":  true,
	}
	return supported[extension]
}

func (a *AudioCompressor) ffmpegArgs(input, output, ext string) []string {
	switch ext {
	case ".mp3":
		return []string{"-i", input, "-c:a", "libmp3lame", "-b:a", "128k", output}
	case ".wav":
		// WAV is uncompressed PCM — re-encode to lower sample rate/bit depth
		return []string{"-i", input, "-ar", "22050", "-sample_fmt", "s16", output}
	case ".flac":
		return []string{"-i", input, "-c:a", "flac", "-compression_level", "8", output}
	case ".ogg":
		return []string{"-i", input, "-c:a", "libvorbis", "-q:a", "3", output}
	case ".m4a", ".aac":
		return []string{"-i", input, "-c:a", "aac", "-b:a", "128k", output}
	default:
		return []string{"-i", input, "-b:a", "128k", output}
	}
}
