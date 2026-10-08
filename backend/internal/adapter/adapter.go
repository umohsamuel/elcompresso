package adapter

import (
	"github.com/umohsamuel/elcompresso/internal/adapter/compress/audio"
	"github.com/umohsamuel/elcompresso/internal/adapter/compress/image"
	"github.com/umohsamuel/elcompresso/internal/adapter/compress/video"
	"github.com/umohsamuel/elcompresso/internal/adapter/ffmpeg"
	"github.com/umohsamuel/elcompresso/internal/domain/compress"
	storageDomain "github.com/umohsamuel/elcompresso/internal/domain/storage"
	"github.com/umohsamuel/elcompresso/pkg/env"
)

type AdapterDependencies struct {
	EnvironmentVariables *env.EnvironmentVariables
	FFmpeg               *ffmpeg.Runner
	Storage              storageDomain.Storage
}

type Adapters struct {
	EnvironmentVariables *env.EnvironmentVariables
	FFmpeg               *ffmpeg.Runner
	Compressors          map[compress.FileType]compress.Interface
	Storage              storageDomain.Storage
}

func NewAdapter(deps AdapterDependencies) *Adapters {
	return &Adapters{
		EnvironmentVariables: deps.EnvironmentVariables,
		FFmpeg:               deps.FFmpeg,
		Compressors: map[compress.FileType]compress.Interface{
			compress.FileTypeVideo: video.NewVideoCompressor(deps.FFmpeg),
			compress.FileTypeAudio: audio.NewAudioCompressor(deps.FFmpeg),
			compress.FileTypeImage: image.NewImageCompressor(deps.FFmpeg, deps.EnvironmentVariables.Limits.ImagePixels),
		},
		Storage: deps.Storage,
	}
}
