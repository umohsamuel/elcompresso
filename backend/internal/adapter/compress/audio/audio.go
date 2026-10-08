package audio

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/umohsamuel/elcompresso/internal/adapter/ffmpeg"
	"github.com/umohsamuel/elcompresso/internal/domain/compress"
)

type AudioCompressor struct {
	ff *ffmpeg.Runner
}

func NewAudioCompressor(ff *ffmpeg.Runner) compress.Interface {
	return &AudioCompressor{ff: ff}
}

var formatInfo = map[string]struct{ ext, contentType string }{
	"mp3":  {"mp3", "audio/mpeg"},
	"aac":  {"m4a", "audio/mp4"},
	"opus": {"opus", "audio/ogg"},
	"ogg":  {"ogg", "audio/ogg"},
	"flac": {"flac", "audio/flac"},
	"wav":  {"wav", "audio/wav"},
}

// sourceFormat maps an input codec to the output format "original" resolves to.
var sourceFormat = map[string]string{
	"mp3":    "mp3",
	"aac":    "aac",
	"opus":   "opus",
	"vorbis": "ogg",
	"flac":   "flac",
}

func (a *AudioCompressor) Compress(ctx context.Context, task compress.Task, progress compress.ProgressFunc) (*compress.Result, error) {
	opts := *task.Options.Audio
	progress("analyzing", 0)

	probe, err := a.ff.Probe(ctx, task.InputPath)
	if err != nil {
		return nil, err
	}
	if probe.Audio == nil {
		return nil, errors.New("no audio track found in this file")
	}

	format := opts.Format
	if format == "original" {
		format = sourceFormat[probe.Audio.Codec]
		if strings.HasPrefix(probe.Audio.Codec, "pcm_") {
			format = "wav"
		}
		if format == "" {
			format = "mp3"
		}
	}
	if !a.ff.HasEncoder(ffmpeg.AudioEncoders[format]) {
		return nil, fmt.Errorf("%s output is not available on this server", format)
	}

	start, length := opts.Trim.Window(probe.Duration)
	if opts.Trim.Active() && probe.Duration > 0 && start >= probe.Duration {
		return nil, fmt.Errorf("trim start (%.1fs) is past the end of the audio (%.1fs)", start, probe.Duration)
	}

	if opts.TrimSilence && length > 0 {
		progress("detecting silence", 0)
		lead, trail, err := a.detectSilence(ctx, task.InputPath, start, length)
		if err != nil {
			return nil, err
		}
		if lead+trail < length {
			start += lead
			length -= lead + trail
		}
	}
	trimmed := opts.Trim.Active() || opts.TrimSilence

	info := formatInfo[format]
	output := filepath.Join(task.WorkDir, "output."+info.ext)

	args := []string{}
	if start > 0 {
		args = append(args, "-ss", ftoa(start))
	}
	args = append(args, "-i", task.InputPath)
	if trimmed && length > 0 {
		args = append(args, "-t", ftoa(length))
	}
	args = append(args, "-map", "0:a:0", "-vn", "-sn", "-dn")

	var filters []string
	if opts.Normalize {
		filters = append(filters, "loudnorm=I=-16:TP=-1.5:LRA=11")
	}
	if len(filters) > 0 {
		args = append(args, "-af", strings.Join(filters, ","))
	}

	args = append(args, codecArgs(format, opts, probe.Audio)...)

	if opts.Channels > 0 && opts.Channels < probe.Audio.Channels {
		args = append(args, "-ac", strconv.Itoa(opts.Channels))
	} else if format == "mp3" && probe.Audio.Channels > 2 {
		args = append(args, "-ac", "2") // LAME is stereo-only
	}

	sampleRate := opts.SampleRate
	if sampleRate == 0 && opts.Normalize {
		sampleRate = probe.Audio.SampleRate // loudnorm upsamples to 192 kHz otherwise
	}
	if sampleRate > 0 && format != "opus" {
		if format == "mp3" && sampleRate > 48000 {
			sampleRate = 48000
		}
		args = append(args, "-ar", strconv.Itoa(sampleRate))
	}

	if opts.StripMetadata {
		args = append(args, "-map_metadata", "-1", "-map_chapters", "-1")
	}
	if format == "aac" {
		args = append(args, "-movflags", "+faststart")
	}
	args = append(args, a.ff.ThreadArgs()...)
	args = append(args, output)

	progress("encoding", 0)
	if err := a.ff.Run(ctx, args, length, func(f float64) { progress("encoding", f) }); err != nil {
		return nil, err
	}

	stat, err := os.Stat(output)
	if err != nil {
		return nil, fmt.Errorf("encoder produced no output: %w", err)
	}
	res := &compress.Result{Path: output, Ext: info.ext, ContentType: info.contentType, Size: stat.Size()}
	if out, err := a.ff.Probe(ctx, output); err == nil {
		res.Info = compress.MediaInfo{Duration: out.Duration, Codec: format}
	}
	return res, nil
}

func codecArgs(format string, opts compress.AudioOptions, src *ffmpeg.AudioStream) []string {
	bitrate := fmt.Sprintf("%dk", opts.Bitrate)
	switch format {
	case "mp3":
		return []string{"-c:a", "libmp3lame", "-b:a", bitrate}
	case "aac":
		return []string{"-c:a", "aac", "-b:a", bitrate}
	case "opus":
		return []string{"-c:a", "libopus", "-b:a", bitrate, "-vbr", "on"}
	case "ogg":
		return []string{"-c:a", "libvorbis", "-b:a", bitrate}
	case "flac":
		args := []string{"-c:a", "flac", "-compression_level", "8"}
		if opts.BitDepth == 16 || (opts.BitDepth == 0 && src.BitsPerSample > 0 && src.BitsPerSample <= 16) {
			args = append(args, "-sample_fmt", "s16")
		}
		return args
	case "wav":
		if opts.BitDepth == 24 || (opts.BitDepth == 0 && src.BitsPerSample > 16) {
			return []string{"-c:a", "pcm_s24le"}
		}
		return []string{"-c:a", "pcm_s16le"}
	}
	return nil
}

var (
	silenceStartRe = regexp.MustCompile(`silence_start: (-?[\d.]+)`)
	silenceEndRe   = regexp.MustCompile(`silence_end: (-?[\d.]+)`)
)

// detectSilence returns how many seconds of silence are at the start and end
// of the selected window. It streams the analysis instead of buffering the
// whole decoded file (unlike an areverse-based approach).
func (a *AudioCompressor) detectSilence(ctx context.Context, input string, start, length float64) (lead, trail float64, err error) {
	const minSilence = 0.3
	args := []string{}
	if start > 0 {
		args = append(args, "-ss", ftoa(start))
	}
	args = append(args, "-i", input, "-t", ftoa(length), "-map", "0:a:0",
		"-af", fmt.Sprintf("silencedetect=noise=-50dB:d=%g", minSilence), "-f", "null", os.DevNull)

	type span struct{ start, end float64 }
	var spans []span
	open := -1.0
	err = a.ff.RunLines(ctx, args, func(line string) {
		if m := silenceStartRe.FindStringSubmatch(line); m != nil {
			open, _ = strconv.ParseFloat(m[1], 64)
		} else if m := silenceEndRe.FindStringSubmatch(line); m != nil && open >= 0 {
			end, _ := strconv.ParseFloat(m[1], 64)
			spans = append(spans, span{open, end})
			open = -1
		}
	})
	if err != nil {
		return 0, 0, err
	}
	if open >= 0 {
		spans = append(spans, span{open, length})
	}
	if len(spans) == 0 {
		return 0, 0, nil
	}

	// Keep a short tail of silence so cuts don't sound abrupt.
	const pad = 0.1
	if first := spans[0]; first.start <= 0.05 {
		lead = max(first.end-pad, 0)
	}
	if last := spans[len(spans)-1]; last.end >= length-0.05 && !(len(spans) == 1 && lead > 0 && last.start <= 0.05) {
		trail = max(length-last.start-pad, 0)
	}
	return lead, trail, nil
}

func ftoa(f float64) string { return strconv.FormatFloat(f, 'f', 3, 64) }
