// Package ffmpeg wraps the ffmpeg and ffprobe binaries: capability detection,
// media probing and running encodes with progress reporting.
package ffmpeg

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Runner struct {
	ffmpeg   string
	ffprobe  string
	threads  int
	encoders map[string]bool
	muxers   map[string]bool
	Version  string
}

func New(ctx context.Context, ffmpegPath, ffprobePath string, threads int) (*Runner, error) {
	r := &Runner{ffmpeg: ffmpegPath, ffprobe: ffprobePath, threads: threads}

	out, err := exec.CommandContext(ctx, r.ffmpeg, "-hide_banner", "-version").Output()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg not usable at %q: %w", r.ffmpeg, err)
	}
	r.Version, _, _ = strings.Cut(string(out), "\n")

	if err := exec.CommandContext(ctx, r.ffprobe, "-hide_banner", "-version").Run(); err != nil {
		return nil, fmt.Errorf("ffprobe not usable at %q: %w", r.ffprobe, err)
	}

	if r.encoders, err = r.listComponents(ctx, "-encoders"); err != nil {
		return nil, err
	}
	if r.muxers, err = r.listComponents(ctx, "-muxers"); err != nil {
		return nil, err
	}
	return r, nil
}

// listComponents parses the table printed by `ffmpeg -encoders` / `-muxers`.
func (r *Runner) listComponents(ctx context.Context, flag string) (map[string]bool, error) {
	out, err := exec.CommandContext(ctx, r.ffmpeg, "-hide_banner", flag).Output()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg %s: %w", flag, err)
	}
	found := map[string]bool{}
	pastHeader := false
	for line := range strings.Lines(string(out)) {
		fields := strings.Fields(line)
		if !pastHeader {
			pastHeader = len(fields) == 1 && strings.HasPrefix(fields[0], "--")
			continue
		}
		if len(fields) >= 2 {
			for name := range strings.SplitSeq(fields[1], ",") {
				found[name] = true
			}
		}
	}
	return found, nil
}

func (r *Runner) HasEncoder(name string) bool { return r.encoders[name] }
func (r *Runner) HasMuxer(name string) bool   { return r.muxers[name] }

// ThreadArgs returns the per-encoder thread cap, if one is configured.
func (r *Runner) ThreadArgs() []string {
	if r.threads <= 0 {
		return nil
	}
	return []string{"-threads", strconv.Itoa(r.threads)}
}

type VideoStream struct {
	Codec  string
	Width  int
	Height int
	FPS    float64
	PixFmt string
}

type AudioStream struct {
	Codec         string
	Channels      int
	SampleRate    int
	BitsPerSample int
}

type Probe struct {
	FormatName string
	Duration   float64
	Video      *VideoStream
	Audio      *AudioStream
}

func (r *Runner) Probe(ctx context.Context, path string) (*Probe, error) {
	out, err := exec.CommandContext(ctx, r.ffprobe,
		"-v", "error", "-print_format", "json", "-show_format", "-show_streams", path,
	).Output()
	if err != nil {
		return nil, errors.New("could not read the file - it may be corrupt or not a supported media file")
	}

	var raw struct {
		Format struct {
			FormatName string `json:"format_name"`
			Duration   string `json:"duration"`
		} `json:"format"`
		Streams []struct {
			CodecType        string `json:"codec_type"`
			CodecName        string `json:"codec_name"`
			Width            int    `json:"width"`
			Height           int    `json:"height"`
			PixFmt           string `json:"pix_fmt"`
			AvgFrameRate     string `json:"avg_frame_rate"`
			RFrameRate       string `json:"r_frame_rate"`
			Channels         int    `json:"channels"`
			SampleRate       string `json:"sample_rate"`
			BitsPerRawSample string `json:"bits_per_raw_sample"`
			BitsPerSample    int    `json:"bits_per_sample"`
			Duration         string `json:"duration"`
			Disposition      struct {
				AttachedPic int `json:"attached_pic"`
			} `json:"disposition"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("parse ffprobe output: %w", err)
	}

	p := &Probe{FormatName: raw.Format.FormatName, Duration: parseFloat(raw.Format.Duration)}
	for _, s := range raw.Streams {
		switch {
		case s.CodecType == "video" && s.Disposition.AttachedPic == 0 && p.Video == nil:
			fps := parseRate(s.AvgFrameRate)
			if fps == 0 {
				fps = parseRate(s.RFrameRate)
			}
			p.Video = &VideoStream{Codec: s.CodecName, Width: s.Width, Height: s.Height, FPS: fps, PixFmt: s.PixFmt}
		case s.CodecType == "audio" && p.Audio == nil:
			bits, _ := strconv.Atoi(s.BitsPerRawSample)
			if bits == 0 {
				bits = s.BitsPerSample
			}
			sr, _ := strconv.Atoi(s.SampleRate)
			p.Audio = &AudioStream{Codec: s.CodecName, Channels: s.Channels, SampleRate: sr, BitsPerSample: bits}
		}
		if p.Duration == 0 {
			p.Duration = parseFloat(s.Duration)
		}
	}
	return p, nil
}

// Run executes ffmpeg with the given arguments. If duration > 0, onProgress is
// called with the fraction of the input processed so far.
func (r *Runner) Run(ctx context.Context, args []string, duration float64, onProgress func(float64)) error {
	full := append([]string{"-hide_banner", "-nostdin", "-y", "-loglevel", "error", "-progress", "pipe:1", "-nostats"}, args...)
	cmd := exec.CommandContext(ctx, r.ffmpeg, full...)
	cmd.WaitDelay = 5 * time.Second

	stderr := &tailBuffer{max: 4096}
	cmd.Stderr = stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	sc := bufio.NewScanner(stdout)
	for sc.Scan() {
		key, val, ok := strings.Cut(sc.Text(), "=")
		if !ok || key != "out_time_us" || duration <= 0 || onProgress == nil {
			continue
		}
		if us, err := strconv.ParseFloat(val, 64); err == nil && us >= 0 {
			onProgress(min(us/1e6/duration, 1))
		}
	}
	_, _ = io.Copy(io.Discard, stdout)

	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("encoding failed: %s", stderr.lastLine())
	}
	return nil
}

// RunLines executes ffmpeg at info log level and hands every stderr line to
// onLine. Used for analysis filters such as silencedetect.
func (r *Runner) RunLines(ctx context.Context, args []string, onLine func(string)) error {
	full := append([]string{"-hide_banner", "-nostdin", "-loglevel", "info", "-nostats"}, args...)
	cmd := exec.CommandContext(ctx, r.ffmpeg, full...)
	cmd.WaitDelay = 5 * time.Second
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	sc := bufio.NewScanner(stderr)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		onLine(sc.Text())
	}
	_, _ = io.Copy(io.Discard, stderr)
	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("analysis failed: %w", err)
	}
	return nil
}

// tailBuffer keeps only the last max bytes written to it.
type tailBuffer struct {
	mu  sync.Mutex
	buf []byte
	max int
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if over := len(t.buf) - t.max; over > 0 {
		t.buf = t.buf[over:]
	}
	return len(p), nil
}

func (t *tailBuffer) lastLine() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	lines := bytes.Split(bytes.TrimSpace(t.buf), []byte("\n"))
	if last := strings.TrimSpace(string(lines[len(lines)-1])); last != "" {
		return last
	}
	return "unknown ffmpeg error"
}

// logPrefixRe matches ffmpeg's "[component @ 0x1234] " log prefixes.
var logPrefixRe = regexp.MustCompile(`\[[^\]]*@ (0x)?[0-9a-fA-F]+\] `)

func parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func parseRate(s string) float64 {
	num, den, ok := strings.Cut(s, "/")
	if !ok {
		return parseFloat(s)
	}
	n, d := parseFloat(num), parseFloat(den)
	if d == 0 {
		return 0
	}
	return n / d
}
