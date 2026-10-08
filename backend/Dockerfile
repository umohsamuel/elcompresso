# syntax=docker/dockerfile:1

FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/elcompresso ./cmd

# Debian trixie ships ffmpeg 7.1 with x264, x265, SVT-AV1, libaom, VP9,
# Opus, LAME, Vorbis and WebP encoders.
FROM debian:trixie-slim
RUN apt-get update \
 && apt-get install -y --no-install-recommends ffmpeg ca-certificates curl tini \
 && rm -rf /var/lib/apt/lists/* \
 && useradd --system --uid 10001 --home /app elcompresso \
 && mkdir -p /data && chown elcompresso /data

WORKDIR /app
COPY --from=build /out/elcompresso /app/elcompresso

USER elcompresso
ENV PORT=:8080 \
    DATA_DIR=/data \
    PRODUCTION_ENVIRONMENT=true
VOLUME /data
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s \
  CMD curl -fsS http://localhost:8080/ping || exit 1

ENTRYPOINT ["/usr/bin/tini", "--", "/app/elcompresso"]
