You know that moment when a project starts simple but ends up forcing you to learn more about FFmpeg? Yeah, welcome to `elcompresso`, a media compression tool that does one thing: accept a file, compress it, upload to S3, and returns a download link (easy enough right?).

This is how i built it.

## Backend

`elcompresso` is a REST API that:

- accepts video, audio, or image uploads
- compresses videos/audio with FFmpeg and images with Go’s native `image` package
- stores the result in AWS S3
- generates a presigned URL for secure downloads

The entire backend is written in Golang & Gin.

## Architecture: Domain, Adapters, Services, and Handlers

The backend follows hexagonal architecture, separating concerns into layers:

```
backend/
├── cmd/
│   ├── main.go           (entry point, dependency wiring)
│   └── api/api.go        (route setup)
├── internal/
│   ├── adapter/          (external services: compression, storage)
│   ├── service/          (business logic layer)
│   ├── port/http/        (HTTP handlers)
│   └── domain/           (interfaces and value objects)
└── pkg/
    ├── env/              (environment config)
    └── response/         (JSON response wrappers)
```

### Wiring It All Together

In `backend/cmd/main.go`, we initialize the entire dependency graph:

```go
func main() {
    environmentVariables := env.LoadEnvironment()

    cfg, err := awsConfig.LoadDefaultConfig(context.TODO())
    if err != nil {
        panic(err)
    }
    s3Client := s3.NewFromConfig(cfg)

    adapterDependencies := adapter.AdapterDependencies{
        EnvironmentVariables: environmentVariables,
        Compressor:           &compress.CompressorDependencies{},
        StorageClient:        s3Client,
    }

    adapters := adapter.NewAdapter(adapterDependencies)
    serviceDependencies := service.ServiceDependencies{
        Adapter: adapters,
    }
    services := service.NewService(serviceDependencies)

    r := api.API(services, environmentVariables)
    r.Engine.Run(environmentVariables.Port)
}
```

Three lines of actual instantiation. Everything else flows from those.

## The API Layer

Routes are defined in `backend/cmd/api/api.go`. We set CORS to allow any origin (reserve your comments mr. security expert), configure file upload memory limits, and expose compression endpoints:

```go
func API(services *service.Services, environment *env.EnvironmentVariables) *Server {
    r := &Server{
        Service:     services,
        Engine:      gin.Default(),
        Environment: environment,
    }

    config := cors.DefaultConfig()
    config.AllowAllOrigins = true
    config.AllowMethods = []string{"POST", "GET", "PUT", "OPTIONS"}

    r.Engine.Use(cors.New(config))
    r.Engine.Static("/downloads", "tmp")
    r.Engine.GET("/health", ...)

    api := r.Engine.Group("/api/v1")
    {
        r.fileCompressRoutes(api)
        r.fileUploadRoutes(api)
    }

    return r
}
```

### Compression Routes

Three POST endpoints at `/api/v1/file-compress/{video,audio,image}`. Each accepts a multipart form with:

- `file`: the media to compress
- `quality`: a 1-100 quality hint (interpreted differently per format)

### Handlers: The Request Pipeline

In `backend/internal/port/http/handler/compress.go`, the `CompressHandler` is initialized with dependencies and processes requests. Let's walk through the video handler:

```go
func (h CompressHandler) CompressVideo(c *gin.Context) {
    var fData FormData

    if err := c.ShouldBind(&fData); err != nil {
        response.NewErrorResponse(fmt.Errorf("invalid form data: %v", err.Error())).Send(c)
        return
    }

    if fData.File.Size > 500<<20 {
        response.NewErrorResponse(fmt.Errorf("file too large: max 500MB")).Send(c)
        return
    }

    f, err := fData.File.Open()
    if err != nil {
        response.NewErrorResponse(fmt.Errorf("failed to open file: %v", err)).Send(c)
        return
    }
    defer f.Close()

    fmtedFileName := strings.ReplaceAll(fData.File.Filename, " ", "_")

    req := compress.CompressionRequest{
        Input:    f,
        FileName: fmtedFileName,
        FileType: "video",
        Quality:  fData.Quality,
    }

    res, err := h.adapter.Compressor.Video.Compress(req)
    if err != nil {
        response.NewErrorResponse(fmt.Errorf("failed to compress file: %v", err)).Send(c)
        return
    }

    key, err := h.adapter.Storage.Upload(c.Request.Context(), uuid.New().String()+"_"+fData.File.Filename, res.Output)
    if err != nil {
        response.NewErrorResponse(fmt.Errorf("upload failed: %w", err)).Send(c)
        return
    }

    dUrl, err := h.adapter.Storage.GenerateDownloadURL(c.Request.Context(), key, 24*time.Hour)

    response.NewSuccessResponse("success", gin.H{
        "original_size":   fData.File.Size,
        "compressed_size": res.CompressedSize,
        "download_link":   dUrl,
    }, nil).Send(c)
}
```

The pattern is identical for audio and image; essentially just parse, validate size, compress, upload, generate download URL, return JSON.

## The Compression Adapters

Here's where the magic happens. Each media type has its own compressor implementing the `compress.Interface`:

```go
type Interface interface {
    Compress(req CompressionRequest) (*CompressionResult, error)
    Supports(fileType FileType, extension string) bool
}
```

### Video Compression

`backend/internal/adapter/compress/video/video.go` uses FFmpeg to handle `.mp4`, `.mkv`, `.avi`, `.mov`, `.webm`, `.flv`.

```go
func (v *VideoCompressor) Compress(req compress.CompressionRequest) (*compress.CompressionResult, error) {
    ext := filepath.Ext(req.FileName)

    inputFile, err := os.CreateTemp("", "input-*"+ext)
    if err != nil {
        return nil, err
    }
    defer os.Remove(inputFile.Name())

    if _, err := io.Copy(inputFile, req.Input); err != nil {
        return nil, err
    }
    inputFile.Close()

    outputFile, err := os.CreateTemp("", "output-*"+ext)
    if err != nil {
        return nil, err
    }
    defer os.Remove(outputFile.Name())
    outputFile.Close()

    args := v.ffmpegArgs(inputFile.Name(), outputFile.Name(), ext)
    cmd := exec.Command("ffmpeg", args...)
    if err := cmd.Run(); err != nil {
        return nil, fmt.Errorf("ffmpeg failed: %w", err)
    }

    compressed, err := os.Open(outputFile.Name())
    if err != nil {
        return nil, err
    }

    info, _ := compressed.Stat()

    return &compress.CompressionResult{
        Output:         compressed,
        CompressedSize: info.Size(),
    }, nil
}
```

The FFmpeg args vary by extension:

```go
func (v *VideoCompressor) ffmpegArgs(input, output, ext string) []string {
    switch ext {
    case ".webm":
        return []string{"-y", "-i", input, "-c:v", "libvpx-vp9", "-crf", "30", "-b:v", "0", "-c:a", "libopus", output}
    case ".flv":
        return []string{"-y", "-i", input, "-c:v", "flv1", "-c:a", "mp3", output}
    default:
        return []string{"-y", "-i", input, "-vcodec", "libx264", "-crf", "28", "-c:a", "aac", output}
    }
}
```

### Audio Compression

`backend/internal/adapter/compress/audio/audio.go` does the same but for audio formats: `.mp3`, `.wav`, `.flac`, `.aac`, `.ogg`, `.m4a`.

Quality is converted to bitrate or sample rate:

```go
func (a *AudioCompressor) ffmpegArgs(input, output, ext string, req compress.CompressionRequest) []string {
    switch ext {
    case ".mp3":
        bitrate := qualityToBitrate(req.Quality)
        return []string{"-y", "-i", input, "-c:a", "libmp3lame", "-b:a", bitrate, output}
    case ".wav":
        sampleRate := qualityToSampleRate(req.Quality)
        return []string{"-y", "-i", input, "-ar", sampleRate, "-sample_fmt", "s16", output}
    case ".flac":
        return []string{"-y", "-i", input, "-c:a", "flac", "-compression_level", "8", output}
    case ".ogg":
        vorbisQ := qualityToVorbis(req.Quality)
        return []string{"-y", "-i", input, "-c:a", "libvorbis", "-q:a", vorbisQ, output}
    case ".m4a", ".aac":
        bitrate := qualityToBitrate(req.Quality)
        return []string{"-y", "-i", input, "-c:a", "aac", "-b:a", bitrate, output}
    default:
        bitrate := qualityToBitrate(req.Quality)
        return []string{"-y", "-i", input, "-b:a", bitrate, output}
    }
}
```

### Image Compression

`backend/internal/adapter/compress/image/image.go` uses the native `image` package.

```go
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
```

JPEG gets quality control; PNG gets maximum compression.

## Storage: S3 with Presigned URLs

The storage adapter (`backend/internal/adapter/storage/storage.go`) wraps the AWS SDK:

```go
type Stg struct {
    Client *s3.Client
    Env    env.EnvironmentVariables
}

func NewStorageClient(deps StgDeps) storage.Storage {
    return &Stg{
        Client: deps.Client,
        Env:    deps.Env,
    }
}

func (s *Stg) Upload(ctx context.Context, filename string, file io.Reader) (string, error) {
    key := "compressed/" + filename

    _, err := s.Client.PutObject(ctx, &s3.PutObjectInput{
        Bucket: aws.String(s.Env.S3.AWS_BUCKET),
        Key:    aws.String(key),
        Body:   file,
    })
    if err != nil {
        return "", err
    }

    return key, nil
}

func (s *Stg) GenerateDownloadURL(ctx context.Context, filename string, expiry time.Duration) (string, error) {
    presignClient := s3.NewPresignClient(s.Client)

    req, err := presignClient.PresignGetObject(ctx,
        &s3.GetObjectInput{
            Bucket: aws.String(s.Env.S3.AWS_BUCKET),
            Key:    aws.String(filename),
        }, s3.WithPresignExpires(expiry))

    if err != nil {
        return "", fmt.Errorf("failed to generate presigned URL: %w", err)
    }

    return req.URL, nil
}
```

Two operations:

1. **Upload**: Write to S3 under `compressed/{filename}`, return the key.
2. **GenerateDownloadURL**: Create a presigned URL valid for 24 hours.

This is the AWS SDK V2 approach (yeah, i had my share of headaches hopping from articles to docs).

## Configuration

Environment variables are loaded in `backend/pkg/env/env.go`:

```go
type EnvironmentVariables struct {
    Port                  string
    ProductionEnvironment bool
    ClientDomain          string
    ProjectName           string
    STORAGE_TYPE          string
    S3                    *S3Config
}

type S3Config struct {
    AWS_REGION            string
    AWS_BUCKET            string
    AWS_ACCESS_KEY_ID     string
    AWS_SECRET_ACCESS_KEY string
}
```

Required at startup:

- `AWS_REGION`
- `AWS_BUCKET`
- `AWS_ACCESS_KEY_ID`
- `AWS_SECRET_ACCESS_KEY`

Optional:

- `PORT` (defaults to `:5000`)
- `PRODUCTION_ENVIRONMENT`
- `CLIENT_DOMAIN`
- `PROJECT_NAME`

## Response Format

Success responses come from `backend/pkg/response/response.go`:

```go
type SuccessResponse struct {
    StatusCode int         `json:"statusCode,omitempty"`
    Message    string      `json:"message"`
    Data       interface{} `json:"data,omitempty"`
    Metadata   interface{} `json:"metadata,omitempty"`
}
```

Error responses include the status code, message, and error detail:

```go
type ErrorResponse struct {
    StatusCode   int    `json:"statusCode"`
    Message      string `json:"message"`
    ErrorMessage any    `json:"error"`
}
```

A successful compression returns:

```json
{
  "message": "success",
  "data": {
    "original_size": 52428800,
    "compressed_size": 15728640,
    "download_link": "https://bucket.s3.region.amazonaws.com/compressed/..."
  }
}
```

## Demo

<video src="https://res.cloudinary.com/db6nohcui/video/upload/v1779196212/compressed-elcompresso-demo_tbxazb.mp4" controls playsinline width="100%"></video>

## Conclusion

Building a media compression backend in Go is pretty straightforward and easy enough.

(If you want to see the full codebase, check the [repository](https://github.com/umohsamuel/elcompresso). you can also play around with the ui.)
