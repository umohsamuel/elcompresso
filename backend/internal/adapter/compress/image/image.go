package image

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

type ImageCompressor struct {
	ff        *ffmpeg.Runner
	maxPixels int64
}

func NewImageCompressor(ff *ffmpeg.Runner, maxPixels int64) compress.Interface {
	return &ImageCompressor{ff: ff, maxPixels: maxPixels}
}

var formatInfo = map[string]struct{ ext, contentType string }{
	"jpeg": {"jpg", "image/jpeg"},
	"png":  {"png", "image/png"},
	"webp": {"webp", "image/webp"},
	"avif": {"avif", "image/avif"},
}

// inputFormats maps ffprobe codec names to the format "original" resolves to.
// Formats we can read but not usefully write fall back to PNG (lossless).
var inputFormats = map[string]string{
	"mjpeg": "jpeg",
	"png":   "png",
	"webp":  "webp",
	"av1":   "avif",
	"gif":   "png",
	"bmp":   "png",
	"tiff":  "png",
}

func (i *ImageCompressor) Compress(ctx context.Context, task compress.Task, progress compress.ProgressFunc) (*compress.Result, error) {
	opts := *task.Options.Image
	progress("analyzing", 0)

	probe, err := i.ff.Probe(ctx, task.InputPath)
	if err != nil {
		return nil, err
	}
	v := probe.Video
	if v == nil || v.Width == 0 || v.Height == 0 {
		return nil, errors.New("this file does not contain a readable image")
	}
	srcFormat, ok := inputFormats[v.Codec]
	if !ok {
		return nil, fmt.Errorf("unsupported image type %q (use JPEG, PNG, WebP, AVIF, GIF, BMP or TIFF)", v.Codec)
	}
	if int64(v.Width)*int64(v.Height) > i.maxPixels {
		return nil, fmt.Errorf("image is too large (%dx%d); the limit is %d megapixels", v.Width, v.Height, i.maxPixels/1_000_000)
	}

	format := opts.Format
	if format == "original" {
		format = srcFormat
	}
	if !i.ff.HasEncoder(ffmpeg.ImageEncoders[format]) {
		return nil, fmt.Errorf("%s output is not available on this server", format)
	}
	alpha := hasAlpha(v.PixFmt)

	// Pre-processing applied before format-specific steps.
	var pre []string
	var input []string
	if v.Codec == "mjpeg" {
		// Apply EXIF orientation ourselves (ffmpeg versions disagree on
		// whether they honour it) since the EXIF block is dropped on output.
		input = append(input, "-noautorotate")
		pre = append(pre, orientationFilters(readJPEGOrientation(task.InputPath))...)
	}
	if opts.MaxWidth > 0 || opts.MaxHeight > 0 {
		w, h := opts.MaxWidth, opts.MaxHeight
		if w == 0 {
			w = maxDim
		}
		if h == 0 {
			h = maxDim
		}
		pre = append(pre, fmt.Sprintf(
			"scale=w='min(%d,iw)':h='min(%d,ih)':force_original_aspect_ratio=decrease:flags=lanczos", w, h))
	}
	input = append(input, "-i", task.InputPath)

	info := formatInfo[format]
	output := filepath.Join(task.WorkDir, "output."+info.ext)

	chain := strings.Join(pre, ",")
	if chain == "" {
		chain = "null"
	}

	var args []string
	switch format {
	case "jpeg":
		graph := "[0:v]" + chain + ",format=yuv420p[out]"
		if alpha {
			// JPEG has no transparency: composite onto white instead of black.
			graph = "[0:v]" + chain + ",split[a][b];[a]drawbox=c=white:t=fill,format=rgb24[bg];[bg][b]overlay=format=auto,format=yuv420p[out]"
		}
		args = []string{"-filter_complex", graph, "-map", "[out]",
			"-c:v", "mjpeg", "-q:v", strconv.Itoa(jpegQScale(opts.Quality)), "-color_range", "pc", "-f", "image2", "-update", "1"}

	case "png":
		graph := "[0:v]" + chain + "[out]"
		if !opts.Lossless && opts.Quality < 100 {
			// Lossy PNG: quantise to a palette (the pngquant/TinyPNG approach).
			graph = fmt.Sprintf("[0:v]%s,split[a][b];[a]palettegen=max_colors=%d:reserve_transparent=%d[p];[b][p]paletteuse=dither=sierra2_4a[out]",
				chain, pngColors(opts.Quality), boolInt(alpha))
		}
		args = []string{"-filter_complex", graph, "-map", "[out]",
			"-c:v", "png", "-compression_level", "9", "-pred", "mixed", "-f", "image2", "-update", "1"}

	case "webp":
		args = []string{"-filter_complex", "[0:v]" + chain + "[out]", "-map", "[out]", "-c:v", "libwebp", "-compression_level", "6"}
		if opts.Lossless {
			args = append(args, "-lossless", "1", "-quality", "100")
		} else {
			pix := "yuv420p"
			if alpha {
				pix = "yuva420p"
			}
			args = append(args, "-quality", strconv.Itoa(opts.Quality), "-pix_fmt", pix)
		}
		args = append(args, "-f", "webp")

	case "avif":
		crf := strconv.Itoa(avifCRF(opts.Quality, opts.Lossless))
		if alpha {
			args = []string{"-filter_complex", "[0:v]" + chain + ",format=yuva444p,split[c][a];[a]alphaextract[al]",
				"-map", "[c]", "-map", "[al]", "-pix_fmt:0", "yuv420p"}
		} else {
			args = []string{"-filter_complex", "[0:v]" + chain + "[out]", "-map", "[out]", "-pix_fmt", "yuv420p"}
		}
		args = append(args, "-c:v", "libaom-av1", "-still-picture", "1", "-crf", crf, "-cpu-used", "6", "-row-mt", "1", "-f", "avif")
	}

	args = append(input, append(args, "-frames:v", "1", "-map_metadata", "-1")...)
	args = append(args, i.ff.ThreadArgs()...)
	args = append(args, output)

	progress("encoding", 0.2)
	if err := i.ff.Run(ctx, args, 0, nil); err != nil {
		return nil, err
	}

	stat, err := os.Stat(output)
	if err != nil {
		return nil, fmt.Errorf("encoder produced no output: %w", err)
	}
	res := &compress.Result{Path: output, Ext: info.ext, ContentType: info.contentType, Size: stat.Size(), Info: compress.MediaInfo{Codec: format}}
	if out, err := i.ff.Probe(ctx, output); err == nil && out.Video != nil {
		res.Info.Width, res.Info.Height = out.Video.Width, out.Video.Height
	}
	return res, nil
}

const maxDim = 100000

func hasAlpha(pixFmt string) bool {
	if pixFmt == "pal8" {
		return true // palettes may carry transparency (GIF/PNG)
	}
	for _, p := range []string{"rgba", "argb", "bgra", "abgr", "yuva", "ya8", "ya16", "gbrap"} {
		if strings.HasPrefix(pixFmt, p) {
			return true
		}
	}
	return false
}

// jpegQScale maps quality 1..100 onto mjpeg's qscale (31 worst .. 2 best).
func jpegQScale(q int) int {
	return int(math.Round(2 + float64(100-q)*29/99))
}

// pngColors maps quality onto palette size on a log scale: 100 -> 256, 60 -> 64.
func pngColors(q int) int {
	return min(max(int(math.Round(math.Pow(2, 3+5*float64(q)/100))), 8), 256)
}

func avifCRF(q int, lossless bool) int {
	if lossless {
		return 0
	}
	return int(math.Round(50 - float64(q-1)*(50-12)/99))
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
