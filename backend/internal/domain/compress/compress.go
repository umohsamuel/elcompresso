package compress

import (
	"context"
	"slices"
)

type FileType string

const (
	FileTypeVideo FileType = "video"
	FileTypeAudio FileType = "audio"
	FileTypeImage FileType = "image"
)

func (t FileType) Valid() bool {
	return t == FileTypeVideo || t == FileTypeAudio || t == FileTypeImage
}

// Task is a single unit of compression work. The compressor reads InputPath and
// writes its output somewhere inside WorkDir.
type Task struct {
	InputPath    string
	WorkDir      string
	OriginalName string
	Options      Options
}

type Result struct {
	Path        string
	Ext         string // without the leading dot
	ContentType string
	Size        int64
	Info        MediaInfo
}

// MediaInfo describes the output so the UI can show what actually happened.
type MediaInfo struct {
	Duration float64 `json:"duration,omitempty"`
	Width    int     `json:"width,omitempty"`
	Height   int     `json:"height,omitempty"`
	Codec    string  `json:"codec,omitempty"`
}

// ProgressFunc reports the current stage and a 0..1 completion fraction.
type ProgressFunc func(stage string, fraction float64)

type Interface interface {
	Compress(ctx context.Context, task Task, progress ProgressFunc) (*Result, error)
}

// Capabilities lists what the running ffmpeg build can actually produce.
type Capabilities struct {
	VideoCodecs     []string `json:"video_codecs"`
	VideoContainers []string `json:"video_containers"`
	AudioFormats    []string `json:"audio_formats"`
	ImageFormats    []string `json:"image_formats"`
}

func (c Capabilities) HasVideoCodec(v string) bool     { return slices.Contains(c.VideoCodecs, v) }
func (c Capabilities) HasVideoContainer(v string) bool { return slices.Contains(c.VideoContainers, v) }
func (c Capabilities) HasAudioFormat(v string) bool    { return slices.Contains(c.AudioFormats, v) }
func (c Capabilities) HasImageFormat(v string) bool    { return slices.Contains(c.ImageFormats, v) }
