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

	args := a.ffmpegArgs(inputFile.Name(), outputFile.Name(), ext, req)
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

func (a *AudioCompressor) ffmpegArgs(input, output, ext string, req compress.CompressionRequest) []string {
	switch ext {
	case ".mp3":
		bitrate := qualityToBitrate(req.Quality)
		return []string{"-y", "-i", input, "-c:a", "libmp3lame", "-b:a", bitrate, output}
	case ".wav":
		sampleRate := qualityToSampleRate(req.Quality)
		return []string{"-y", "-i", input, "-ar", sampleRate, "-sample_fmt", "s16", output}
	case ".flac":
		return []string{"-y", "-i", input, "-c:a", "flac", "-compression_level", "8", output}
	case ".ogg":
		vorbisQ := qualityToVorbis(req.Quality)
		return []string{"-y", "-i", input, "-c:a", "libvorbis", "-q:a", vorbisQ, output}
	case ".m4a", ".aac":
		bitrate := qualityToBitrate(req.Quality)
		return []string{"-y", "-i", input, "-c:a", "aac", "-b:a", bitrate, output}
	default:
		bitrate := qualityToBitrate(req.Quality)
		return []string{"-y", "-i", input, "-b:a", bitrate, output}
	}
}

func qualityToBitrate(quality int) string {
	if quality <= 0 || quality > 100 {
		quality = 50
	}
	kbps := 32 + (quality * 288 / 100)
	return fmt.Sprintf("%dk", kbps)
}

func qualityToSampleRate(quality int) string {
	if quality <= 0 || quality > 100 {
		quality = 50
	}
	rates := []string{"8000", "11025", "16000", "22050", "32000", "44100"}
	idx := quality * (len(rates) - 1) / 100
	return rates[idx]
}

func qualityToVorbis(quality int) string {
	if quality <= 0 || quality > 100 {
		quality = 50
	}
	q := quality * 10 / 100
	return fmt.Sprintf("%d", q)
}
