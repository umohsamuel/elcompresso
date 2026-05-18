package image

import (
	"bytes"
	"fmt"
	goimage "image"
	"image/jpeg"
	"image/png"
	"io"
	"path/filepath"
	"strings"

	"github.com/umohsamuel/elcompresso/internal/domain/compress"
)

type ImageCompressor struct{}

func NewImageCompressor() compress.Interface {
	return &ImageCompressor{}
}

func (i *ImageCompressor) Compress(req compress.CompressionRequest) (*compress.CompressionResult, error) {
	ext := strings.ToLower(filepath.Ext(req.FileName))

	data, err := io.ReadAll(req.Input)
	if err != nil {
		return nil, fmt.Errorf("failed to read input: %w", err)
	}
	originalSize := int64(len(data))

	img, _, err := goimage.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	var buf bytes.Buffer

	switch ext {
	case ".jpg", ".jpeg":
		quality := req.Quality
		if quality <= 0 || quality > 100 {
			quality = 60
		}
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality})
	case ".png":
		encoder := &png.Encoder{CompressionLevel: png.BestCompression}
		err = encoder.Encode(&buf, img)
	default:
		return nil, fmt.Errorf("unsupported image format: %s", ext)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to encode image: %w", err)
	}

	return &compress.CompressionResult{
		Output:         bytes.NewReader(buf.Bytes()),
		OriginalSize:   originalSize,
		CompressedSize: int64(buf.Len()),
		Format:         ext,
	}, nil
}

func (i *ImageCompressor) Supports(fileType compress.FileType, extension string) bool {
	if fileType != compress.FileTypeImage {
		return false
	}
	supported := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		// ".webp": true,
	}
	return supported[extension]
}
