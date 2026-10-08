package compress_test

import (
	"context"
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/umohsamuel/elcompresso/internal/adapter/compress/audio"
	"github.com/umohsamuel/elcompresso/internal/adapter/compress/image"
	"github.com/umohsamuel/elcompresso/internal/adapter/compress/video"
	"github.com/umohsamuel/elcompresso/internal/adapter/ffmpeg"
	"github.com/umohsamuel/elcompresso/internal/domain/compress"
)

// These tests drive the real ffmpeg binary. They are skipped when ffmpeg is
// not installed, and with -short.

func runner(t *testing.T) *ffmpeg.Runner {
	t.Helper()
	if testing.Short() {
		t.Skip("integration test")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	r, err := ffmpeg.New(context.Background(), "ffmpeg", "ffprobe", 0)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func generate(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	out := filepath.Join(dir, name)
	cmd := exec.Command("ffmpeg", append(append([]string{"-v", "error", "-y"}, args...), out)...)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate %s: %v\n%s", name, err, b)
	}
	return out
}

func run(t *testing.T, c compress.Interface, input string, opts compress.Options) *compress.Result {
	t.Helper()
	work := t.TempDir()
	var last float64
	res, err := c.Compress(context.Background(), compress.Task{InputPath: input, WorkDir: work, OriginalName: filepath.Base(input), Options: opts},
		func(_ string, f float64) {
			if f < 0 || f > 1 {
				t.Errorf("progress out of range: %v", f)
			}
			last = f
		})
	if err != nil {
		t.Fatalf("compress: %v", err)
	}
	if res.Size <= 0 {
		t.Fatalf("empty output")
	}
	_ = last
	return res
}

func validated[T interface {
	Validate(compress.Capabilities) error
}](t *testing.T, r *ffmpeg.Runner, o T) T {
	t.Helper()
	if err := o.Validate(r.Capabilities()); err != nil {
		t.Skipf("not supported by this ffmpeg build: %v", err)
	}
	return o
}

func TestVideo(t *testing.T) {
	r := runner(t)
	dir := t.TempDir()
	landscape := generate(t, dir, "land.mp4", "-f", "lavfi", "-i", "testsrc2=s=1280x720:d=3:r=30",
		"-f", "lavfi", "-i", "sine=d=3", "-shortest", "-c:v", "libx264", "-preset", "ultrafast", "-c:a", "aac")
	portrait := generate(t, dir, "port.mov", "-f", "lavfi", "-i", "testsrc2=s=722x1282:d=2:r=60,crop=721:1281", "-c:v", "libx264", "-preset", "ultrafast")
	c := video.NewVideoCompressor(r)

	base := compress.DefaultVideoOptions()
	base.Speed = "fast"

	cases := []struct {
		name   string
		input  string
		mutate func(*compress.VideoOptions)
		check  func(*testing.T, *compress.Result)
	}{
		{"h264 default", landscape, func(o *compress.VideoOptions) {}, nil},
		{"720p->480p no audio", landscape, func(o *compress.VideoOptions) { o.Resolution = 480; o.RemoveAudio = true }, func(t *testing.T, r *compress.Result) {
			if r.Info.Width != 854 || r.Info.Height != 480 {
				t.Errorf("got %dx%d, want 854x480", r.Info.Width, r.Info.Height)
			}
		}},
		{"portrait odd size, 360p, 30fps", portrait, func(o *compress.VideoOptions) { o.Resolution = 360; o.FPS = 30 }, func(t *testing.T, r *compress.Result) {
			if r.Info.Width != 360 || r.Info.Height != 640 {
				t.Errorf("got %dx%d, want 360x640", r.Info.Width, r.Info.Height)
			}
		}},
		{"no upscale", landscape, func(o *compress.VideoOptions) { o.Resolution = 1080 }, func(t *testing.T, r *compress.Result) {
			if r.Info.Height != 720 {
				t.Errorf("upscaled to height %d", r.Info.Height)
			}
		}},
		{"trim", landscape, func(o *compress.VideoOptions) { o.Trim = compress.Trim{Start: 1, End: 2} }, func(t *testing.T, r *compress.Result) {
			if r.Info.Duration < 0.8 || r.Info.Duration > 1.3 {
				t.Errorf("duration %.2f, want ~1s", r.Info.Duration)
			}
		}},
		{"h264 target size two-pass", landscape, func(o *compress.VideoOptions) { o.Mode = compress.ModeSize; o.TargetSizeMB = 0.3 }, func(t *testing.T, r *compress.Result) {
			if float64(r.Size) > 0.3*1024*1024*1.1 {
				t.Errorf("size %d exceeds target", r.Size)
			}
		}},
		{"h265 mov", landscape, func(o *compress.VideoOptions) { o.Codec = "h265"; o.Container = "mov" }, nil},
		{"h265 target size", landscape, func(o *compress.VideoOptions) { o.Codec = "h265"; o.Mode = compress.ModeSize; o.TargetSizeMB = 0.3 }, nil},
		{"vp9 webm", landscape, func(o *compress.VideoOptions) { o.Codec = "vp9"; o.Container = "webm"; o.Resolution = 360 }, nil},
		{"vp9 target size", landscape, func(o *compress.VideoOptions) {
			o.Codec = "vp9"
			o.Container = "webm"
			o.Resolution = 360
			o.Mode = compress.ModeSize
			o.TargetSizeMB = 0.2
		}, nil},
		{"av1 mp4", landscape, func(o *compress.VideoOptions) { o.Codec = "av1"; o.Resolution = 360 }, nil},
		{"av1 webm target size", landscape, func(o *compress.VideoOptions) {
			o.Codec = "av1"
			o.Container = "webm"
			o.Resolution = 360
			o.Mode = compress.ModeSize
			o.TargetSizeMB = 0.2
		}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := base
			tc.mutate(&o)
			validated(t, r, &o)
			res := run(t, c, tc.input, compress.Options{Video: &o})
			if tc.check != nil {
				tc.check(t, res)
			}
		})
	}

	t.Run("target too small", func(t *testing.T) {
		o := base
		o.Mode = compress.ModeSize
		o.TargetSizeMB = 0.001
		_, err := c.Compress(context.Background(), compress.Task{InputPath: landscape, WorkDir: t.TempDir(), Options: compress.Options{Video: &o}}, func(string, float64) {})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestAudio(t *testing.T) {
	r := runner(t)
	dir := t.TempDir()
	// 1s silence + 2s tone + 1s silence, 24-bit 48 kHz stereo WAV.
	wav := generate(t, dir, "in.wav", "-f", "lavfi", "-i",
		"aevalsrc='if(between(t,1,3),0.5*sin(2*PI*440*t),0)|if(between(t,1,3),0.5*sin(2*PI*440*t),0)':s=48000:d=4",
		"-c:a", "pcm_s24le")
	videoIn := generate(t, dir, "clip.mp4", "-f", "lavfi", "-i", "testsrc2=s=320x240:d=2", "-f", "lavfi", "-i", "sine=d=2",
		"-shortest", "-c:v", "libx264", "-preset", "ultrafast", "-c:a", "aac")
	c := audio.NewAudioCompressor(r)

	cases := []struct {
		name   string
		input  string
		mutate func(*compress.AudioOptions)
		check  func(*testing.T, *compress.Result)
	}{
		{"mp3", wav, func(o *compress.AudioOptions) {}, nil},
		{"aac mono 22k", wav, func(o *compress.AudioOptions) { o.Format = "aac"; o.Channels = 1; o.SampleRate = 22050 }, nil},
		{"opus", wav, func(o *compress.AudioOptions) { o.Format = "opus"; o.Bitrate = 64 }, nil},
		{"ogg normalize", wav, func(o *compress.AudioOptions) { o.Format = "ogg"; o.Normalize = true }, nil},
		{"flac 16-bit", wav, func(o *compress.AudioOptions) { o.Format = "flac"; o.BitDepth = 16 }, nil},
		{"wav original keeps 24-bit", wav, func(o *compress.AudioOptions) { o.Format = "original" }, func(t *testing.T, r *compress.Result) {
			if r.Ext != "wav" {
				t.Errorf("ext %s", r.Ext)
			}
		}},
		{"trim silence", wav, func(o *compress.AudioOptions) { o.TrimSilence = true }, func(t *testing.T, r *compress.Result) {
			if r.Info.Duration < 1.9 || r.Info.Duration > 2.5 {
				t.Errorf("duration %.2f, want ~2.2s", r.Info.Duration)
			}
		}},
		{"manual trim", wav, func(o *compress.AudioOptions) { o.Trim = compress.Trim{Start: 0.5, End: 1.5} }, func(t *testing.T, r *compress.Result) {
			if r.Info.Duration < 0.9 || r.Info.Duration > 1.15 {
				t.Errorf("duration %.2f, want ~1s", r.Info.Duration)
			}
		}},
		{"extract from video", videoIn, func(o *compress.AudioOptions) { o.StripMetadata = true }, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := compress.DefaultAudioOptions()
			tc.mutate(&o)
			validated(t, r, &o)
			res := run(t, c, tc.input, compress.Options{Audio: &o})
			if tc.check != nil {
				tc.check(t, res)
			}
		})
	}
}

func TestImage(t *testing.T) {
	r := runner(t)
	dir := t.TempDir()
	photo := generate(t, dir, "photo.jpg", "-f", "lavfi", "-i", "testsrc2=s=1602x902,format=rgb24,crop=1601:901", "-frames:v", "1", "-q:v", "2")
	alpha := generate(t, dir, "alpha.png", "-f", "lavfi", "-i", "testsrc2=s=642x482,format=rgba,crop=641:481,geq=r='r(X,Y)':g='g(X,Y)':b='b(X,Y)':a='if(gt(X,320),0,255)'", "-frames:v", "1")
	gif := generate(t, dir, "anim.gif", "-f", "lavfi", "-i", "testsrc2=s=200x100:d=1:r=5")
	rotated := withOrientation(t, photo, filepath.Join(dir, "rotated.jpg"), 6)
	c := image.NewImageCompressor(r, 100_000_000)

	cases := []struct {
		name   string
		input  string
		mutate func(*compress.ImageOptions)
		check  func(*testing.T, *compress.Result)
	}{
		{"jpeg original", photo, func(o *compress.ImageOptions) {}, func(t *testing.T, r *compress.Result) {
			if r.Ext != "jpg" {
				t.Errorf("ext %s", r.Ext)
			}
		}},
		{"jpeg resize", photo, func(o *compress.ImageOptions) { o.MaxWidth = 800 }, func(t *testing.T, r *compress.Result) {
			if r.Info.Width != 800 || r.Info.Height != 450 {
				t.Errorf("got %dx%d, want 800x450", r.Info.Width, r.Info.Height)
			}
		}},
		{"exif orientation applied", rotated, func(o *compress.ImageOptions) {}, func(t *testing.T, r *compress.Result) {
			if r.Info.Width != 901 || r.Info.Height != 1601 {
				t.Errorf("got %dx%d, want 901x1601 (rotated)", r.Info.Width, r.Info.Height)
			}
		}},
		{"png lossy palette", alpha, func(o *compress.ImageOptions) { o.Quality = 60 }, nil},
		{"png lossless", alpha, func(o *compress.ImageOptions) { o.Lossless = true }, nil},
		{"png alpha to jpeg", alpha, func(o *compress.ImageOptions) { o.Format = "jpeg" }, nil},
		{"webp lossy alpha", alpha, func(o *compress.ImageOptions) { o.Format = "webp" }, nil},
		{"webp lossless", photo, func(o *compress.ImageOptions) { o.Format = "webp"; o.Lossless = true }, nil},
		{"avif odd size", photo, func(o *compress.ImageOptions) { o.Format = "avif"; o.MaxHeight = 301 }, nil},
		{"avif alpha", alpha, func(o *compress.ImageOptions) { o.Format = "avif" }, nil},
		{"gif first frame", gif, func(o *compress.ImageOptions) {}, func(t *testing.T, r *compress.Result) {
			if r.Ext != "png" {
				t.Errorf("ext %s", r.Ext)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := compress.DefaultImageOptions()
			tc.mutate(&o)
			validated(t, r, &o)
			res := run(t, c, tc.input, compress.Options{Image: &o})
			if tc.check != nil {
				tc.check(t, res)
			}
		})
	}
}

// withOrientation copies a JPEG, inserting a minimal EXIF block with the given
// orientation right after the SOI marker.
func withOrientation(t *testing.T, src, dst string, orientation uint16) string {
	t.Helper()
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	tiff := []byte{'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0, 0x12, 0x01, 3, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}
	binary.LittleEndian.PutUint16(tiff[18:], orientation)
	payload := append([]byte("Exif\x00\x00"), tiff...)
	app1 := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(app1[2:], uint16(len(payload)+2))
	out := append(append(append([]byte{}, b[:2]...), append(app1, payload...)...), b[2:]...)
	if err := os.WriteFile(dst, out, 0o644); err != nil {
		t.Fatal(err)
	}
	return dst
}
