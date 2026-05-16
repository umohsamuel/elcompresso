package compress

import "io"

type FileType string

const (
	FileTypeVideo FileType = "video"
	FileTypeAudio FileType = "audio"
	FileTypeImage FileType = "image"
)

type CompressionRequest struct {
	Input    io.Reader `json:"Input"`
	FileName string    `json:"FileName"`
	FileType FileType  `json:"FileType"`
	Quality  int       `json:"Quality"`
}

type CompressionResult struct {
	Output         io.Reader `json:"Output"`
	OriginalSize   int64     `json:"OriginalSize"`
	CompressedSize int64     `json:"CompressedSize"`
	Format         string    `json:"Format"`
}
