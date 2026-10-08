package compress

import (
	"encoding/json"
	"fmt"
	"slices"
)

// Options holds exactly one of the per-type option sets.
type Options struct {
	Video *VideoOptions `json:"video,omitempty"`
	Audio *AudioOptions `json:"audio,omitempty"`
	Image *ImageOptions `json:"image,omitempty"`
}

// Trim selects a time range in seconds. End == 0 means "until the end".
type Trim struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

func (t *Trim) normalize() error {
	if t.Start < 0 || t.End < 0 {
		return fmt.Errorf("trim times cannot be negative")
	}
	if t.End > 0 && t.End <= t.Start {
		return fmt.Errorf("trim end must be after trim start")
	}
	return nil
}

// Window resolves the trim against the media duration, returning the start
// offset and the length of the selected range (0 if unknown).
func (t Trim) Window(duration float64) (start, length float64) {
	end := duration
	if t.End > 0 && (duration == 0 || t.End < duration) {
		end = t.End
	}
	if end == 0 {
		return t.Start, 0
	}
	return t.Start, max(end-t.Start, 0)
}

func (t Trim) Active() bool { return t.Start > 0 || t.End > 0 }

const (
	ModeQuality = "quality"
	ModeSize    = "size"
)

type VideoOptions struct {
	Codec         string  `json:"codec"`          // h264 | h265 | av1 | vp9
	Container     string  `json:"container"`      // mp4 | webm | mkv | mov
	Mode          string  `json:"mode"`           // quality | size
	Quality       int     `json:"quality"`        // 1..100, higher is better
	TargetSizeMB  float64 `json:"target_size_mb"` // used when Mode == size
	Speed         string  `json:"speed"`          // fast | balanced | slow
	Resolution    int     `json:"resolution"`     // short side in px, 0 keeps original
	FPS           int     `json:"fps"`            // 0 keeps original
	RemoveAudio   bool    `json:"remove_audio"`
	AudioBitrate  int     `json:"audio_bitrate"` // kbps
	Trim          Trim    `json:"trim"`
	StripMetadata bool    `json:"strip_metadata"`
}

type AudioOptions struct {
	Format        string `json:"format"`      // original | mp3 | aac | opus | ogg | flac | wav
	Bitrate       int    `json:"bitrate"`     // kbps, lossy formats only
	Channels      int    `json:"channels"`    // 0 keeps original, 1 mono, 2 stereo
	SampleRate    int    `json:"sample_rate"` // 0 keeps original
	BitDepth      int    `json:"bit_depth"`   // 0 keeps original, 16 or 24 (wav/flac)
	Normalize     bool   `json:"normalize"`   // EBU R128 loudness normalisation
	TrimSilence   bool   `json:"trim_silence"`
	Trim          Trim   `json:"trim"`
	StripMetadata bool   `json:"strip_metadata"`
}

type ImageOptions struct {
	Format    string `json:"format"` // original | jpeg | png | webp | avif
	Quality   int    `json:"quality"`
	Lossless  bool   `json:"lossless"`
	MaxWidth  int    `json:"max_width"`
	MaxHeight int    `json:"max_height"`
}

func DefaultVideoOptions() VideoOptions {
	return VideoOptions{
		Codec:         "h264",
		Container:     "mp4",
		Mode:          ModeQuality,
		Quality:       60,
		Speed:         "balanced",
		AudioBitrate:  128,
		StripMetadata: true,
	}
}

func DefaultAudioOptions() AudioOptions {
	return AudioOptions{Format: "mp3", Bitrate: 128, StripMetadata: false}
}

func DefaultImageOptions() ImageOptions {
	return ImageOptions{Format: "original", Quality: 75}
}

// ContainerCodecs lists which video codecs each container can carry.
var ContainerCodecs = map[string][]string{
	"mp4":  {"h264", "h265", "av1"},
	"mov":  {"h264", "h265"},
	"mkv":  {"h264", "h265", "av1", "vp9"},
	"webm": {"vp9", "av1"},
}

var (
	videoResolutions = []int{0, 2160, 1440, 1080, 720, 540, 480, 360, 240}
	videoFPS         = []int{0, 60, 50, 30, 25, 24, 15, 12}
	audioSampleRates = []int{0, 48000, 44100, 32000, 24000, 22050, 16000, 11025, 8000}
)

const (
	maxDimension = 16384
	maxTargetMB  = 100_000
)

func (o *VideoOptions) Validate(c Capabilities) error {
	if !c.HasVideoCodec(o.Codec) {
		return fmt.Errorf("video codec %q is not available", o.Codec)
	}
	if !c.HasVideoContainer(o.Container) {
		return fmt.Errorf("container %q is not available", o.Container)
	}
	if !slices.Contains(ContainerCodecs[o.Container], o.Codec) {
		return fmt.Errorf("%s cannot be stored in a .%s file", o.Codec, o.Container)
	}
	switch o.Mode {
	case ModeQuality:
		o.Quality = clamp(o.Quality, 1, 100)
	case ModeSize:
		if o.TargetSizeMB <= 0 || o.TargetSizeMB > maxTargetMB {
			return fmt.Errorf("target size must be between 0 and %d MB", maxTargetMB)
		}
	default:
		return fmt.Errorf("mode must be %q or %q", ModeQuality, ModeSize)
	}
	if !slices.Contains([]string{"fast", "balanced", "slow"}, o.Speed) {
		return fmt.Errorf("speed must be fast, balanced or slow")
	}
	if !slices.Contains(videoResolutions, o.Resolution) {
		return fmt.Errorf("unsupported resolution %d", o.Resolution)
	}
	if !slices.Contains(videoFPS, o.FPS) {
		return fmt.Errorf("unsupported frame rate %d", o.FPS)
	}
	if !o.RemoveAudio {
		o.AudioBitrate = clamp(o.AudioBitrate, 32, 320)
	}
	return o.Trim.normalize()
}

var (
	audioFormats      = []string{"original", "mp3", "aac", "opus", "ogg", "flac", "wav"}
	LossyAudioFormats = []string{"mp3", "aac", "opus", "ogg"}
	audioBitDepths    = []int{0, 16, 24}
)

func (o *AudioOptions) Validate(c Capabilities) error {
	if !slices.Contains(audioFormats, o.Format) || (o.Format != "original" && !c.HasAudioFormat(o.Format)) {
		return fmt.Errorf("audio format %q is not available", o.Format)
	}
	o.Bitrate = clamp(o.Bitrate, 8, 320)
	if o.Channels < 0 || o.Channels > 2 {
		return fmt.Errorf("channels must be 0 (original), 1 or 2")
	}
	if !slices.Contains(audioSampleRates, o.SampleRate) {
		return fmt.Errorf("unsupported sample rate %d", o.SampleRate)
	}
	if !slices.Contains(audioBitDepths, o.BitDepth) {
		return fmt.Errorf("bit depth must be 0 (original), 16 or 24")
	}
	return o.Trim.normalize()
}

var imageFormats = []string{"original", "jpeg", "png", "webp", "avif"}

func (o *ImageOptions) Validate(c Capabilities) error {
	if !slices.Contains(imageFormats, o.Format) || (o.Format != "original" && !c.HasImageFormat(o.Format)) {
		return fmt.Errorf("image format %q is not available", o.Format)
	}
	o.Quality = clamp(o.Quality, 1, 100)
	if o.MaxWidth < 0 || o.MaxHeight < 0 || o.MaxWidth > maxDimension || o.MaxHeight > maxDimension {
		return fmt.Errorf("max width/height must be between 0 and %d", maxDimension)
	}
	return nil
}

// ParseOptions decodes the client's JSON options for the given file type,
// applies defaults for anything omitted and validates the result.
func ParseOptions(t FileType, raw []byte, c Capabilities) (Options, error) {
	decode := func(v any) error {
		if len(raw) == 0 {
			return nil
		}
		if err := json.Unmarshal(raw, v); err != nil {
			return fmt.Errorf("invalid options: %w", err)
		}
		return nil
	}

	switch t {
	case FileTypeVideo:
		o := DefaultVideoOptions()
		if err := decode(&o); err != nil {
			return Options{}, err
		}
		return Options{Video: &o}, o.Validate(c)
	case FileTypeAudio:
		o := DefaultAudioOptions()
		if err := decode(&o); err != nil {
			return Options{}, err
		}
		return Options{Audio: &o}, o.Validate(c)
	case FileTypeImage:
		o := DefaultImageOptions()
		if err := decode(&o); err != nil {
			return Options{}, err
		}
		return Options{Image: &o}, o.Validate(c)
	}
	return Options{}, fmt.Errorf("unknown file type %q", t)
}

func clamp(v, lo, hi int) int {
	return min(max(v, lo), hi)
}
