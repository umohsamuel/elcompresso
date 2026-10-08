package ffmpeg

import "github.com/umohsamuel/elcompresso/internal/domain/compress"

// Encoder names used for each user-facing codec/format, in order of preference.
var (
	VideoEncoders = map[string][]string{
		"h264": {"libx264"},
		"h265": {"libx265"},
		"av1":  {"libsvtav1", "libaom-av1"},
		"vp9":  {"libvpx-vp9"},
	}
	ContainerMuxers = map[string]string{
		"mp4":  "mp4",
		"mov":  "mov",
		"mkv":  "matroska",
		"webm": "webm",
	}
	AudioEncoders = map[string]string{
		"mp3":  "libmp3lame",
		"aac":  "aac",
		"opus": "libopus",
		"ogg":  "libvorbis",
		"flac": "flac",
		"wav":  "pcm_s16le",
	}
	ImageEncoders = map[string]string{
		"jpeg": "mjpeg",
		"png":  "png",
		"webp": "libwebp",
		"avif": "libaom-av1",
	}
)

// VideoEncoder returns the best available encoder for a codec, or "".
func (r *Runner) VideoEncoder(codec string) string {
	for _, enc := range VideoEncoders[codec] {
		if r.HasEncoder(enc) {
			return enc
		}
	}
	return ""
}

func (r *Runner) Capabilities() compress.Capabilities {
	var c compress.Capabilities
	for _, codec := range []string{"h264", "h265", "av1", "vp9"} {
		if r.VideoEncoder(codec) != "" {
			c.VideoCodecs = append(c.VideoCodecs, codec)
		}
	}
	for _, container := range []string{"mp4", "webm", "mkv", "mov"} {
		if r.HasMuxer(ContainerMuxers[container]) {
			c.VideoContainers = append(c.VideoContainers, container)
		}
	}
	for _, f := range []string{"mp3", "aac", "opus", "ogg", "flac", "wav"} {
		if r.HasEncoder(AudioEncoders[f]) {
			c.AudioFormats = append(c.AudioFormats, f)
		}
	}
	for _, f := range []string{"jpeg", "png", "webp", "avif"} {
		if !r.HasEncoder(ImageEncoders[f]) {
			continue
		}
		if f == "avif" && !r.HasMuxer("avif") {
			continue
		}
		c.ImageFormats = append(c.ImageFormats, f)
	}
	return c
}
