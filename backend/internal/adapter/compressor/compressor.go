package compressor

import (
	"github.com/umohsamuel/elcompresso/internal/adapter/compressor/audio"
	"github.com/umohsamuel/elcompresso/internal/adapter/compressor/image"
	"github.com/umohsamuel/elcompresso/internal/adapter/compressor/video"
	"github.com/umohsamuel/elcompresso/internal/domain/compress"
)

type CompressorDependencies struct {
}

type Compressors struct {
	Audio compress.Interface
	Image compress.Interface
	Video compress.Interface
}

func NewCompressors(deps CompressorDependencies) *Compressors {
	return &Compressors{
		Audio: audio.NewAudioCompressor(),
		Image: image.NewImageCompressor(),
		Video: video.NewVideoCompressor(),
	}
}
