# elcompresso

File compression service for video, audio, and image files. Upload files, compress them with configurable quality settings, and download the results via presigned URLs.

## Overview

elcompresso is a full-stack application that provides file compression capabilities for three media types: video, audio, and image. The backend processes files using FFmpeg for video and audio, Go's standard library for images, and stores compressed results in AWS S3. The frontend offers a tabbed interface for selecting compression type and quality.

## Tech Stack

Backend:

- Go 1.21+
- Gin Web Framework
- FFmpeg (video/audio compression)
- AWS SDK v2 (S3 storage)
- Hexagonal Architecture (adapters, domain, services)

Frontend:

- Next.js 16+ with React 19
- TypeScript
- TanStack Query (React Query)
- Tailwind CSS v4 with OKLch
- shadcn/ui components
- Bun (package manager)

## Prerequisites

Backend:

- Go 1.21 or later
- FFmpeg installed and in system PATH
- AWS credentials (for S3 access)
- An AWS S3 bucket configured for public object ownership

Frontend:

- Node.js 18+ or Bun 1.0+
- Git

## Architecture

Backend follows clean hexagonal architecture:

```
backend/
  cmd/main.go                      (entry point)
  cmd/api/api.go                   (route setup)
  internal/
    domain/                        (business logic, interfaces)
    adapter/                       (external service implementations)
      compressor/                  (video, audio, image compression)
      storage/                     (S3 upload, presigned URLs)
    service/                       (application layer)
    port/http/handler/             (HTTP request handlers)
  pkg/
    env/                           (environment loading)
    response/                      (response formatting)
```

Frontend structure:

```
ui/
  src/
    app/                           (Next.js app router)
    components/                    (React components)
    hooks/                         (custom hooks)
    lib/                           (utilities, API client)
    types/                         (TypeScript types)
    constants/                     (API routes, query keys)
```

## Backend Setup

Extract the backend archive and navigate to the backend directory.

Install dependencies:

```bash
cd backend
go mod download
```

Install FFmpeg:

Windows (using Chocolatey):

```bash
choco install ffmpeg
```

macOS:

```bash
brew install ffmpeg
```

Linux (Ubuntu/Debian):

```bash
sudo apt-get install ffmpeg
```

Verify FFmpeg is in PATH:

```bash
ffmpeg -version
```

### Environment Configuration

Create a `.env` file in the backend directory or copy from `.env.example`:

```bash
PORT=:8080
PRODUCTION_ENVIRONMENT=false
CLIENT_DOMAIN=localhost
PROJECT_NAME=elcompresso
STORAGE_TYPE=s3
AWS_REGION=eu-north-1
AWS_BUCKET=your-s3-bucket-name
AWS_ACCESS_KEY_ID=your-access-key
AWS_SECRET_ACCESS_KEY=your-secret-key
```

Obtain AWS credentials by:

1. Log in to AWS Console
2. Navigate to IAM > Users > Your User > Security Credentials
3. Create Access Keys under Programmatic Access
4. Store the Access Key ID and Secret Access Key

S3 bucket configuration:

1. Create a new S3 bucket in your desired region
2. Under Object Ownership, set to "Bucket owner enforced" (simplifies permissions)
3. Set a lifecycle rule to auto-delete objects after X days (optional but recommended)

Running the backend:

Development mode (with hot reload using air):

```bash
cd backend
air
```

Production build:

```bash
cd backend
go build -o elcompresso ./cmd/main.go
./elcompresso
```

The backend runs on http://localhost:8080 and serves routes at `/api/v1/`.

## Frontend Setup

Navigate to the UI directory:

```bash
cd ui
```

Install dependencies using Bun:

```bash
bun install
```

Or using npm:

```bash
npm install
```

Environment configuration:

Create `.env` in the ui directory:

```bash
NEXT_PUBLIC_API_URL=http://localhost:8080
```

For production, set NEXT_PUBLIC_API_URL to your backend domain.

Running the frontend:

Development mode:

```bash
bun dev
```

Or with npm:

```bash
npm run dev
```

The frontend runs on http://localhost:3000.

Production build:

```bash
bun run build
bun start
```

## Running Both Services

Terminal 1 (Backend):

```bash
cd backend
air
```

Terminal 2 (Frontend):

```bash
cd ui
bun dev
```

Access the application at http://localhost:3000.

## API Documentation

Base URL: http://localhost:8080

All endpoints use multipart/form-data and return JSON.

### Compression Endpoints

POST /api/v1/file-compress/video

- Max size: 500MB
- Supported formats: MP4, MKV, AVI, MOV, WebM, FLV
- Form fields: file (required), quality (1-100, optional, defaults to 50)
- Response:

```json
{
  "message": "success",
  "data": {
    "original_size": 42998994,
    "compressed_size": 8029701,
    "download_link": "https://bucket.s3.region.amazonaws.com/..."
  }
}
```

POST /api/v1/file-compress/audio

- Max size: 100MB
- Supported formats: MP3, WAV, FLAC, AAC, OGG, M4A
- Form fields: file (required), quality (1-100, optional)
- Audio quality mapping: 1 = 32kbps, 100 = 320kbps

POST /api/v1/file-compress/image

- Max size: 100MB
- Supported formats: JPEG, PNG
- PNG files are converted to JPEG for compression (lossless compression provides minimal savings)
- Form fields: file (required), quality (1-100, optional)
- Quality directly maps to JPEG quality parameter

Download links are presigned URLs valid for 24 hours.

### Utility Endpoints

GET /health

- Response: {"message": "server up!!!"}

GET /ping

- Response: {"message": "pong"}

## File Size Reference

Quality parameter interpretation:

Video: CRF (Constant Rate Factor) 1-28 (lower = better quality, larger file)

- Quality 10 = ~1/5 of original
- Quality 50 = ~1/3 of original
- Quality 90 = ~2/3 of original

Audio: Bitrate scaling 32kbps to 320kbps

- Quality 10 = 32kbps (lo-fi)
- Quality 50 = 176kbps (balanced)
- Quality 100 = 320kbps (high quality)

Image: JPEG quality 1-100

- Quality 10 = highly compressed
- Quality 50 = moderate compression
- Quality 100 = minimal compression

## S3 Lifecycle Rules

Set auto-deletion of compressed files to avoid storage costs:

AWS Console > S3 > Your Bucket > Management > Lifecycle Rules

- Apply to: compressed/ prefix
- Expiration: 7 days after creation

Or via AWS CLI:

```bash
aws s3api put-bucket-lifecycle-configuration \
  --bucket your-bucket \
  --lifecycle-configuration '{
    "Rules": [{
      "ID": "DeleteAfter7Days",
      "Status": "Enabled",
      "Filter": {},
      "Expiration": {"Days": 7}
    }]
  }'
```

## Development

Backend code organization:

- domain/: Interfaces and business logic, independent of frameworks
- adapter/: External service implementations (S3, FFmpeg)
- service/: Application layer coordinating adapters and domain logic
- port/http: HTTP handlers translating requests to service layer

Frontend code organization:

- components/: Reusable React components
- hooks/: Custom React hooks for mutation/query logic
- lib/api: API client and request functions
- constants/: Query keys and mutation keys for TanStack Query
- types/: Shared TypeScript interfaces

## Troubleshooting

FFmpeg not found:
Ensure FFmpeg is installed and accessible in system PATH. Run ffmpeg -version to verify.

S3 Access Denied:
Check AWS credentials in .env. Verify IAM user has s3:PutObject permission on the bucket.

S3 ACL errors:
If you see "AccessControlListNotSupported", your bucket has Object Ownership set to "Bucket owner enforced". The backend correctly handles this. No ACL parameters are used.

CORS errors in frontend:
Backend has CORS enabled for all origins. If errors persist, check:

1. NEXT_PUBLIC_API_URL points to correct backend
2. Backend is running on the configured port
3. No firewall blocking requests

## Notes

Compression times depend on file size and quality setting:

- Video (100MB at quality 50): 30-90 seconds
- Audio (50MB at quality 50): 10-30 seconds
- Image (10MB at quality 50): 1-5 seconds

Frontend includes a loading modal with elapsed time display during compression.

## License

MIT
