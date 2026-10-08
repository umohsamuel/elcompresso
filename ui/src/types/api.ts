export type FileType = "video" | "audio" | "image";

export type JobStatus = "queued" | "processing" | "done" | "failed" | "canceled";

export interface Trim {
  start: number;
  end: number;
}

export type VideoCodec = "h264" | "h265" | "av1" | "vp9";
export type VideoContainer = "mp4" | "webm" | "mkv" | "mov";
export type Speed = "fast" | "balanced" | "slow";

export interface VideoOptions {
  codec: VideoCodec;
  container: VideoContainer;
  mode: "quality" | "size";
  quality: number;
  target_size_mb: number;
  speed: Speed;
  resolution: number;
  fps: number;
  remove_audio: boolean;
  audio_bitrate: number;
  trim: Trim;
  strip_metadata: boolean;
}

export type AudioFormat = "original" | "mp3" | "aac" | "opus" | "ogg" | "flac" | "wav";

export interface AudioOptions {
  format: AudioFormat;
  bitrate: number;
  channels: number;
  sample_rate: number;
  bit_depth: number;
  normalize: boolean;
  trim_silence: boolean;
  trim: Trim;
  strip_metadata: boolean;
}

export type ImageFormat = "original" | "jpeg" | "png" | "webp" | "avif";

export interface ImageOptions {
  format: ImageFormat;
  quality: number;
  lossless: boolean;
  max_width: number;
  max_height: number;
}

export interface OptionsByType {
  video: VideoOptions;
  audio: AudioOptions;
  image: ImageOptions;
}

export interface MediaInfo {
  duration?: number;
  width?: number;
  height?: number;
  codec?: string;
}

export interface JobResult {
  name: string;
  size: number;
  content_type: string;
  saved_percent: number;
  larger: boolean;
  download_url: string;
  info: MediaInfo;
}

export interface Job {
  id: string;
  type: FileType;
  status: JobStatus;
  stage?: string;
  progress: number;
  queue_position?: number;
  error?: string;
  original_name: string;
  original_size: number;
  result?: JobResult;
  created_at: string;
  expires_at?: string;
}

export interface Capabilities {
  formats: {
    video_codecs: VideoCodec[];
    video_containers: VideoContainer[];
    audio_formats: Exclude<AudioFormat, "original">[];
    image_formats: Exclude<ImageFormat, "original">[];
  };
  limits: {
    video_bytes: number;
    audio_bytes: number;
    image_bytes: number;
    image_megapixels: number;
    max_active_jobs: number;
    result_ttl_seconds: number;
  };
  container_codecs: Record<VideoContainer, VideoCodec[]>;
  defaults: OptionsByType;
}

export interface ApiEnvelope<T> {
  message: string;
  data: T;
}

export interface ApiError {
  message: string;
  error: string;
}
