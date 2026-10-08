# elcompresso

Self-hosted media compression for video, audio and images. Batch upload files, pick a preset or fine-tune every option, watch real encoding progress, and download the results.

- **Backend**: Go + FFmpeg, async job queue, runs on a single EC2 instance (Docker + Caddy for HTTPS).
- **Frontend**: Next.js, deploys to Vercel.

## Features

**Video**: H.264, H.265/HEVC, AV1 or VP9 in MP4, WebM, MKV or MOV.
- Compress by **quality** (CRF, with a labelled slider showing the real encoder value) or to an exact **target size** (two-pass), e.g. "fit Discord's 10 MB".
- Max resolution (480p to 4K; applies to the short side, so vertical video works too, and it never upscales), frame-rate cap, encoding speed.
- **Remove audio**, audio bitrate, **trim** start/end, strip metadata (GPS, device info), MP4 fast-start for web playback.
- Presets: Balanced, Discord 10 MB, WhatsApp 16 MB, Email 25 MB, Web (VP9), Smallest (AV1), Archive.

**Audio**: MP3, AAC, Opus, OGG Vorbis, FLAC, WAV, or keep the input format.
- Bitrate slider (32-320 kbps), mono/stereo, sample rate, 16/24-bit for lossless.
- **Loudness normalisation** (EBU R128, -16 LUFS), **trim silence** at start/end, manual trim, strip tags.
- **Extract audio from video**: drop a video on the Audio tab.

**Image**: JPEG, PNG, WebP, AVIF, or keep the input format (reads GIF/BMP/TIFF too).
- Quality slider, lossless mode, **lossy PNG** via palette quantisation (the TinyPNG approach).
- Resize to fit a max width/height, never upscaling.
- EXIF orientation applied, then all EXIF/GPS metadata removed. Transparency is preserved (WebP/AVIF/PNG) or flattened onto white for JPEG.
- Before/after comparison slider. Paste images with Ctrl+V.

**Platform**
- Batch processing with per-file upload and encode progress, queue position, cancel, retry, and "download all".
- Jobs survive a page refresh. Results expire automatically (default 1 h).
- The server reports what its FFmpeg build supports; the UI only offers options that will work.
- Per-IP rate limiting and concurrency caps, a bounded queue with backpressure, per-job timeouts, and graceful shutdown.
- Local disk or S3 storage. Settings are remembered per browser.

## Architecture

```
Browser (Vercel)                     EC2
┌────────────────┐   HTTPS   ┌──────────────────────────────────────────┐
│ Next.js UI     │──────────▶│ Caddy (TLS) ─▶ elcompresso API (Go)      │
│ - upload       │           │                 ├─ job queue + N workers │
│ - poll status  │           │                 ├─ ffmpeg / ffprobe      │
│ - download     │◀──────────│                 └─ local disk or S3      │
└────────────────┘           └──────────────────────────────────────────┘
```

1. `POST /api/v1/jobs/{video|audio|image}` streams the upload to disk and returns `202` with a job id.
2. A worker probes the file, runs FFmpeg and reports progress.
3. The client polls `GET /api/v1/jobs?ids=…` and downloads from `GET /api/v1/jobs/{id}/download`.

The backend follows a hexagonal layout:

```
backend/
  cmd/main.go                    wiring, graceful shutdown
  cmd/api/api.go                 routes, CORS, logging
  internal/domain/               option types + validation, job model, storage interface
  internal/adapter/ffmpeg/       probing, capability detection, progress-reporting runner
  internal/adapter/compress/     video, audio, image pipelines (+ integration tests)
  internal/adapter/storage/      local disk and S3
  internal/service/jobs.go       queue, workers, limits, expiry
  internal/port/http/            handlers, rate limiting
deploy/                          docker-compose, Caddyfile, EC2 setup script
ui/                              Next.js frontend
```

## Local development

Requirements: Go 1.26+, FFmpeg (with ffprobe) on your PATH, Node 20+ and pnpm (or bun/npm).

```bash
# Backend: http://localhost:8080
cd backend
cp .env.example .env        # defaults work as-is (local storage)
go run ./cmd                # or `air` for hot reload

# Frontend: http://localhost:3000
cd ui
cp .env.example .env        # NEXT_PUBLIC_API_URL=http://localhost:8080
pnpm install
pnpm dev
```

Tests (the integration tests run real FFmpeg encodes and are skipped if it isn't installed):

```bash
cd backend && go test ./...        # add -short to skip FFmpeg tests
```

## Deploying

### Backend on EC2

1. **Launch an instance**: Ubuntu 24.04, x86_64 or Graviton. Video encoding is CPU bound; a `c7i.large`/`c7g.large` (2 vCPU) is a reasonable start, `c7i.xlarge` or larger for heavier use. Give it 30 GB+ of disk (uploads and results are stored there temporarily).
2. **Security group**: allow inbound 22 (your IP only), 80 and 443.
3. **Elastic IP**: allocate one and attach it.
4. **DNS**: create an `A` record such as `api.yourdomain.com` pointing at the Elastic IP. If you don't have a domain, `<ip-with-dashes>.sslip.io` (e.g. `13-50-1-2.sslip.io`) works and still gets a real certificate.
5. **Install and configure**:
   ```bash
   ssh ubuntu@<elastic-ip>
   git clone https://github.com/umohsamuel/elcompresso.git && cd elcompresso
   bash deploy/ec2-setup.sh          # installs Docker, adds swap, creates deploy/.env
   nano deploy/.env                  # set API_DOMAIN and ALLOWED_ORIGINS
   cd deploy && sudo docker compose --profile caddy up -d --build
   curl https://api.yourdomain.com/health
   ```
   Caddy obtains and renews the TLS certificate automatically.

**Updating**: `git pull && cd deploy && sudo docker compose --profile caddy up -d --build`.

#### Shared instance behind host nginx (GitHub Actions)

The live deploy shares an instance with other apps whose host nginx + certbot already own ports 80/443, so Caddy is left out (it's behind the `caddy` compose profile). The API binds to `127.0.0.1:9003` and `deploy/nginx.conf` proxies the API domain to it. Processing limits in `deploy/limits.env` are sized for that box (2 vCPU, ~1 GB free RAM, 20 GB disk): 500 MB uploads, one encode at a time, 2 GB swap added by the workflow.

`.github/workflows/deploy-backend.yml` runs on pushes to `prod` that touch `backend/` or `deploy/` (or manually). It runs the tests, builds the image on the runner, streams it to the instance over SSH, writes `deploy/.env`, starts the container, installs the nginx site and a certbot certificate on the first deploy, and checks `https://<API_DOMAIN>/health`.

It reads the GitHub `production` environment:

```bash
gh api -X PUT repos/umohsamuel/elcompresso/environments/production
gh secret set EC2_HOST --env production --body "<elastic-ip>"
gh secret set EC2_SSH_KEY --env production < ~/.ssh/<key>
gh variable set API_DOMAIN --env production --body "elcompresso-api.umohsg.com"
gh variable set ALLOWED_ORIGINS --env production --body "https://elcompresso.umohsg.com,https://elcompresso-*.vercel.app"
```

The API domain needs a DNS `A` record pointing at the instance before the first deploy, or certbot fails.

**Logs**: `sudo docker compose logs -f api`.

HTTPS is required: browsers block an HTTPS page on Vercel from calling an `http://` API (mixed content).

#### S3: direct uploads and results (recommended)

With `STORAGE_TYPE=s3`, browsers upload straight to S3 with a presigned POST form and download results from presigned URLs, so file bytes never pass through the API, and queued jobs don't use the instance's disk:

1. The browser calls `POST /api/v1/uploads/{type}` with the file's name and size. The API checks the limits and returns a presigned form (valid 1 hour). The size limit is part of the signed policy, so S3 itself rejects bigger files.
2. The browser posts the file to S3, showing upload progress.
3. The browser calls `POST /api/v1/jobs/{type}` with `{"upload_id", "options"}`. Each upload can be claimed once, by the client that started it, and its real size is checked.
4. When a worker starts the job it pulls the file from S3, encodes it, uploads the result to `compressed/`, and deletes the upload.

Local storage (`STORAGE_TYPE=local`) keeps the plain multipart upload through the API, so local development needs no AWS account.

Setup (the JSON files are in `deploy/s3/`):

1. **Bucket**: create one in the instance's region with all public access blocked.
2. **CORS** (bucket → Permissions → CORS): paste `cors.json`, with your frontend origins.
3. **Lifecycle** (bucket → Management): add the rules in `lifecycle.json`, which expire uploads that were never turned into jobs, and old results, after a day.
4. **IAM role**: create a role for EC2 with the policy in `instance-policy.json` (replace `BUCKET_NAME`), then attach it to the instance (Actions → Security → Modify IAM role). No access keys are needed.
5. **Metadata hop limit**: containers reach the instance role through one extra network hop, so set it to 2 (Actions → Instance settings → Modify instance metadata options → hop limit `2`).
6. **Config**: set `AWS_BUCKET` and `AWS_REGION` (GitHub `production` variables for the Actions deploy, or `deploy/.env` by hand). The deploy then switches to `STORAGE_TYPE=s3`.

### Frontend on Vercel

1. Import the repo in Vercel and set **Root Directory** to `ui` (the framework is detected as Next.js).
2. Add the environment variable `NEXT_PUBLIC_API_URL=https://api.yourdomain.com`, for Production and Preview.
3. Deploy, then make sure the Vercel URL is in the backend's `ALLOWED_ORIGINS`. A wildcard such as `https://elcompresso-*.vercel.app` covers preview deployments.

## Configuration

All backend settings are environment variables (see `backend/.env.example` and `deploy/.env.example`).

| Variable | Default | Description |
| --- | --- | --- |
| `PORT` | `:8080` | Listen address |
| `PRODUCTION_ENVIRONMENT` | `false` | JSON logs, Gin release mode |
| `ALLOWED_ORIGINS` | `http://localhost:3000` | Comma-separated CORS origins; one `*` wildcard per entry allowed; `*` alone allows all |
| `TRUSTED_PROXIES` | private ranges | Proxies whose `X-Forwarded-For` is trusted |
| `DATA_DIR` | `./data` | Working files, plus results when using local storage |
| `STORAGE_TYPE` | `local` | `local` or `s3` |
| `AWS_REGION`, `AWS_BUCKET`, `S3_PREFIX`, `S3_UPLOAD_PREFIX` | -, -, `compressed/`, `uploads/` | S3 settings |
| `RESULT_TTL` | `1h` | How long results stay downloadable |
| `WORKERS` | CPUs / 2 | Parallel encodes |
| `QUEUE_SIZE` | `100` | Max waiting jobs before returning 503 |
| `JOB_TIMEOUT` | `30m` | Max processing time per file |
| `FFMPEG_THREADS` | `0` (auto) | Threads per FFmpeg process |
| `FFMPEG_PATH`, `FFPROBE_PATH` | `ffmpeg`, `ffprobe` | Binaries |
| `MAX_VIDEO_MB` / `MAX_AUDIO_MB` / `MAX_IMAGE_MB` | `1024` / `200` / `50` | Upload limits |
| `MAX_IMAGE_MEGAPIXELS` | `100` | Guards against decompression bombs |
| `RATE_LIMIT_PER_MINUTE` | `30` | New jobs per client IP per minute (0 disables) |
| `MAX_ACTIVE_JOBS_PER_IP` | `5` | Unfinished jobs per client IP |

Frontend: `NEXT_PUBLIC_API_URL`, the API's base URL.

Job state is held in memory, so restarting the server drops unfinished jobs (and clears the work directory). That's fine for a single instance; running several instances behind a load balancer would need a shared job store such as Redis.

## API

Responses are JSON: `{"message": "...", "data": ...}` on success, `{"message": "...", "error": "..."}` on failure.

| Method & path | Description |
| --- | --- |
| `GET /api/v1/capabilities` | Available codecs/formats, limits, defaults, and `direct_upload` |
| `POST /api/v1/uploads/{type}` | JSON `{name, size}`. Returns a presigned S3 form `{id, url, fields}` (S3 storage only) |
| `POST /api/v1/jobs/{type}` | Multipart `options` (JSON) + `file`, or JSON `{upload_id, options}` after a direct upload. Returns `202` with the job |
| `GET /api/v1/jobs/{id}` | Job status |
| `GET /api/v1/jobs?ids=a,b,c` | Several jobs at once (unknown ids omitted) |
| `DELETE /api/v1/jobs/{id}` | Cancel a running job, or delete a finished result |
| `GET /api/v1/jobs/{id}/download` | The result file (`?inline=1` to display it in the browser) |
| `GET /health`, `GET /ping` | Health checks |

Example:

```bash
curl -F 'options={"codec":"h265","resolution":720,"remove_audio":true}' \
     -F file=@clip.mov https://api.yourdomain.com/api/v1/jobs/video
```

Options (all optional; omitted fields use the defaults):

```jsonc
// video
{ "codec": "h264|h265|av1|vp9", "container": "mp4|webm|mkv|mov",
  "mode": "quality|size", "quality": 1-100, "target_size_mb": 10,
  "speed": "fast|balanced|slow", "resolution": 0|240|360|480|540|720|1080|1440|2160,
  "fps": 0|12|15|24|25|30|50|60, "remove_audio": false, "audio_bitrate": 128,
  "trim": { "start": 0, "end": 0 }, "strip_metadata": true }

// audio
{ "format": "original|mp3|aac|opus|ogg|flac|wav", "bitrate": 128,
  "channels": 0|1|2, "sample_rate": 0|8000…48000, "bit_depth": 0|16|24,
  "normalize": false, "trim_silence": false, "trim": { "start": 0, "end": 0 },
  "strip_metadata": false }

// image
{ "format": "original|jpeg|png|webp|avif", "quality": 1-100, "lossless": false,
  "max_width": 0, "max_height": 0 }
```

Job statuses: `queued`, `processing` (with `stage` and `progress` 0-100), `done` (with `result`), `failed` (with `error`), `canceled`.

## License

MIT
