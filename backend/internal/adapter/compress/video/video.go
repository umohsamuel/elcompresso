package video

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/umohsamuel/elcompresso/internal/adapter/ffmpeg"
	"github.com/umohsamuel/elcompresso/internal/domain/compress"
)

type VideoCompressor struct {
	ff *ffmpeg.Runner
}

func NewVideoCompressor(ff *ffmpeg.Runner) compress.Interface {
	return &VideoCompressor{ff: ff}
}

var containerInfo = map[string]struct{ ext, contentType string }{
	"mp4":  {"mp4", "video/mp4"},
	"mov":  {"mov", "video/quicktime"},
	"mkv":  {"mkv", "video/x-matroska"},
	"webm": {"webm", "video/webm"},
}

// crfRange is the CRF used at quality 100 (best) and quality 1 (worst).
var crfRange = map[string][2]float64{
	"libx264":    {18, 35},
	"libx265":    {20, 38},
	"libsvtav1":  {22, 55},
	"libaom-av1": {22, 55},
	"libvpx-vp9": {24, 52},
}

func (v *VideoCompressor) Compress(ctx context.Context, task compress.Task, progress compress.ProgressFunc) (*compress.Result, error) {
	opts := *task.Options.Video
	progress("analyzing", 0)

	probe, err := v.ff.Probe(ctx, task.InputPath)
	if err != nil {
		return nil, err
	}
	if probe.Video == nil || probe.Video.Width == 0 {
		return nil, errors.New("no video track found in this file")
	}

	start, length := opts.Trim.Window(probe.Duration)
	if opts.Trim.Active() && probe.Duration > 0 && start >= probe.Duration {
		return nil, fmt.Errorf("trim start (%.1fs) is past the end of the video (%.1fs)", start, probe.Duration)
	}

	encoder := v.ff.VideoEncoder(opts.Codec)
	if encoder == "" {
		return nil, fmt.Errorf("codec %s is not available on this server", opts.Codec)
	}
	audioCodec := "aac"
	if opts.Container == "webm" {
		audioCodec = "libopus"
	}
	keepAudio := !opts.RemoveAudio && probe.Audio != nil

	info := containerInfo[opts.Container]
	output := filepath.Join(task.WorkDir, "output."+info.ext)

	input := []string{}
	if start > 0 {
		input = append(input, "-ss", ftoa(start))
	}
	input = append(input, "-i", task.InputPath)
	if opts.Trim.End > 0 && length > 0 {
		input = append(input, "-t", ftoa(length))
	}
	input = append(input, "-map", "0:v:0", "-vf", v.filters(opts, probe))

	videoArgs := []string{"-c:v", encoder, "-pix_fmt", "yuv420p"}
	videoArgs = append(videoArgs, speedArgs(encoder, opts.Speed)...)
	if encoder == "libx265" && (opts.Container == "mp4" || opts.Container == "mov") {
		videoArgs = append(videoArgs, "-tag:v", "hvc1") // required for Apple players
	}
	videoArgs = append(videoArgs, v.ff.ThreadArgs()...)

	var audioArgs []string
	if keepAudio {
		audioArgs = []string{"-map", "0:a:0", "-c:a", audioCodec, "-b:a", fmt.Sprintf("%dk", opts.AudioBitrate)}
		if probe.Audio.Channels > 2 {
			audioArgs = append(audioArgs, "-ac", "2")
		}
	} else {
		audioArgs = []string{"-an"}
	}

	outArgs := []string{"-sn", "-dn"}
	if opts.StripMetadata {
		outArgs = append(outArgs, "-map_metadata", "-1", "-map_chapters", "-1")
	}
	if opts.Container == "mp4" || opts.Container == "mov" {
		outArgs = append(outArgs, "-movflags", "+faststart")
	}

	report := func(stage string, from, span float64) func(float64) {
		return func(f float64) { progress(stage, from+f*span) }
	}

	if opts.Mode == compress.ModeQuality {
		args := concat(input, videoArgs, crfArgs(encoder, opts.Quality), audioArgs, outArgs, []string{output})
		progress("encoding", 0)
		if err := v.ff.Run(ctx, args, length, report("encoding", 0, 1)); err != nil {
			return nil, err
		}
	} else {
		if length <= 0 {
			return nil, errors.New("cannot target a file size: the video duration is unknown")
		}
		audioKbps := 0
		if keepAudio {
			audioKbps = opts.AudioBitrate
		}
		videoKbps, err := targetBitrate(opts.TargetSizeMB, length, audioKbps)
		if err != nil {
			return nil, err
		}
		rate := []string{"-b:v", fmt.Sprintf("%dk", videoKbps)}

		if pass, ok := twoPassArgs(encoder, task.WorkDir); ok {
			pass1 := concat(input, videoArgs, rate, pass(1), []string{"-an", "-f", "null", os.DevNull})
			progress("analyzing (pass 1/2)", 0)
			if err := v.ff.Run(ctx, pass1, length, report("analyzing (pass 1/2)", 0, 0.4)); err != nil {
				return nil, err
			}
			pass2 := concat(input, videoArgs, rate, pass(2), audioArgs, outArgs, []string{output})
			if err := v.ff.Run(ctx, pass2, length, report("encoding (pass 2/2)", 0.4, 0.6)); err != nil {
				return nil, err
			}
		} else {
			vbv := []string{"-maxrate", fmt.Sprintf("%dk", videoKbps*3/2), "-bufsize", fmt.Sprintf("%dk", videoKbps*2)}
			if strings.Contains(encoder, "av1") {
				vbv = nil // SVT-AV1/libaom use their own VBR rate control
			}
			args := concat(input, videoArgs, rate, vbv, audioArgs, outArgs, []string{output})
			progress("encoding", 0)
			if err := v.ff.Run(ctx, args, length, report("encoding", 0, 1)); err != nil {
				return nil, err
			}
		}
	}

	stat, err := os.Stat(output)
	if err != nil {
		return nil, fmt.Errorf("encoder produced no output: %w", err)
	}

	res := &compress.Result{Path: output, Ext: info.ext, ContentType: info.contentType, Size: stat.Size()}
	if out, err := v.ff.Probe(ctx, output); err == nil && out.Video != nil {
		res.Info = compress.MediaInfo{Duration: out.Duration, Width: out.Video.Width, Height: out.Video.Height, Codec: opts.Codec}
	}
	return res, nil
}

func (v *VideoCompressor) filters(opts compress.VideoOptions, probe *ffmpeg.Probe) string {
	var f []string
	if opts.FPS > 0 && probe.Video.FPS > float64(opts.FPS)+0.5 {
		f = append(f, fmt.Sprintf("fps=%d", opts.FPS))
	}
	if opts.Resolution > 0 {
		// Cap the *short* side so portrait and landscape both work. These
		// expressions are evaluated after ffmpeg auto-rotates phone footage,
		// and never upscale.
		t := opts.Resolution
		f = append(f, fmt.Sprintf(
			"scale=w='if(gt(iw,ih),-2,trunc(min(%d,iw)/2)*2)':h='if(gt(iw,ih),trunc(min(%d,ih)/2)*2,-2)':flags=lanczos", t, t))
	} else {
		f = append(f, "scale=trunc(iw/2)*2:trunc(ih/2)*2") // yuv420p needs even dimensions
	}
	return strings.Join(f, ",")
}

func crfArgs(encoder string, quality int) []string {
	r := crfRange[encoder]
	crf := math.Round(r[1] - float64(quality-1)*(r[1]-r[0])/99)
	args := []string{"-crf", strconv.Itoa(int(crf))}
	if encoder == "libvpx-vp9" || encoder == "libaom-av1" {
		args = append(args, "-b:v", "0") // constant-quality mode
	}
	return args
}

func speedArgs(encoder, speed string) []string {
	idx := map[string]int{"fast": 0, "balanced": 1, "slow": 2}[speed]
	switch encoder {
	case "libx264", "libx265":
		return []string{"-preset", []string{"veryfast", "medium", "slow"}[idx]}
	case "libsvtav1":
		return []string{"-preset", []string{"10", "8", "5"}[idx]}
	case "libaom-av1":
		return []string{"-cpu-used", []string{"8", "6", "4"}[idx], "-row-mt", "1"}
	case "libvpx-vp9":
		return []string{"-deadline", "good", "-cpu-used", []string{"5", "3", "1"}[idx], "-row-mt", "1"}
	}
	return nil
}

// twoPassArgs returns a builder for per-pass arguments for encoders that
// support two-pass rate control through ffmpeg.
func twoPassArgs(encoder, workDir string) (func(int) []string, bool) {
	log := filepath.Join(workDir, "passlog")
	switch encoder {
	case "libx264", "libvpx-vp9":
		return func(n int) []string {
			return []string{"-pass", strconv.Itoa(n), "-passlogfile", log}
		}, true
	case "libx265":
		return func(n int) []string {
			return []string{"-x265-params", fmt.Sprintf("pass=%d:stats=%s", n, x265Path(log+".x265"))}
		}, true
	}
	return nil, false
}

// x265Path escapes characters that would break -x265-params parsing.
func x265Path(p string) string {
	return strings.NewReplacer(`\`, `/`, ":", `\:`).Replace(p)
}

// targetBitrate computes the video bitrate (kbps) that lands the output at
// roughly targetMB, leaving room for audio and container overhead.
func targetBitrate(targetMB, seconds float64, audioKbps int) (int, error) {
	totalKbps := targetMB * 1024 * 1024 * 8 * 0.96 / seconds / 1000
	videoKbps := int(totalKbps) - audioKbps
	const minKbps = 50
	if videoKbps < minKbps {
		needMB := float64(minKbps+audioKbps) * 1000 * seconds / 8 / 0.96 / 1024 / 1024
		return 0, fmt.Errorf("target size too small for %.0fs of video: try at least %.1f MB, remove audio, or trim it", seconds, math.Ceil(needMB*10)/10)
	}
	return videoKbps, nil
}

func ftoa(f float64) string { return strconv.FormatFloat(f, 'f', 3, 64) }

func concat(parts ...[]string) []string {
	var out []string
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}
