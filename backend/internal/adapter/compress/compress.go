package compress

import (
	"github.com/umohsamuel/elcompresso/internal/adapter/compress/audio"
	"github.com/umohsamuel/elcompresso/internal/adapter/compress/image"
	"github.com/umohsamuel/elcompresso/internal/adapter/compress/video"
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
