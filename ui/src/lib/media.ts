import type {
  AudioOptions,
  Capabilities,
  FileType,
  ImageOptions,
  OptionsByType,
  VideoCodec,
  VideoContainer,
  VideoOptions,
} from "@/types/api";

/* ---------- File type detection ---------- */

export const ACCEPT: Record<FileType, string> = {
  video: "video/*,.mkv,.flv,.wmv,.m4v,.ts,.3gp",
  // Video files are accepted too: the audio track is extracted.
  audio: "audio/*,video/*,.flac,.opus,.m4a,.wma,.aiff",
  image: "image/jpeg,image/png,image/webp,image/avif,image/gif,image/bmp,image/tiff,.avif",
};

const EXTENSIONS: Record<FileType, string[]> = {
  video: ["mp4", "mkv", "mov", "avi", "webm", "flv", "wmv", "m4v", "ts", "mts", "3gp", "mpg", "mpeg", "ogv"],
  audio: ["mp3", "wav", "flac", "aac", "ogg", "oga", "opus", "m4a", "wma", "aiff", "aif", "amr"],
  image: ["jpg", "jpeg", "png", "webp", "avif", "gif", "bmp", "tif", "tiff"],
};

export function detectType(file: File): FileType | null {
  const major = file.type.split("/")[0];
  if (major === "video" || major === "audio" || major === "image") return major;
  const ext = file.name.split(".").pop()?.toLowerCase() ?? "";
  for (const type of ["video", "audio", "image"] as FileType[]) {
    if (EXTENSIONS[type].includes(ext)) return type;
  }
  return null;
}

/** Whether a file can be processed by the given tab. */
export function acceptsFile(tab: FileType, file: File): boolean {
  const type = detectType(file);
  return type === tab || (tab === "audio" && type === "video");
}

export function sizeLimit(caps: Capabilities | undefined, type: FileType): number {
  if (!caps) return Infinity;
  return caps.limits[`${type}_bytes`];
}

/* ---------- Quality zones ---------- */

export interface Zone {
  from: number;
  label: string;
  hint: string;
}

export const VIDEO_ZONES: Zone[] = [
  { from: 1, label: "Tiny", hint: "Very small files, visible blockiness" },
  { from: 26, label: "Small", hint: "Fine for previews and phones" },
  { from: 46, label: "Balanced", hint: "Good for sharing and streaming" },
  { from: 71, label: "High", hint: "Hard to tell apart from the original" },
  { from: 91, label: "Pristine", hint: "Near-lossless, big files" },
];

export const IMAGE_ZONES: Zone[] = [
  { from: 1, label: "Tiny", hint: "Thumbnails and placeholders" },
  { from: 31, label: "Small", hint: "Noticeable artifacts on close look" },
  { from: 56, label: "Web", hint: "The sweet spot for websites" },
  { from: 81, label: "High", hint: "Photography and print previews" },
  { from: 94, label: "Max", hint: "Minimal savings over the original" },
];

export const AUDIO_BITRATES = [32, 48, 64, 96, 128, 160, 192, 256, 320];

export const AUDIO_ZONES: Zone[] = [
  { from: 32, label: "Voice", hint: "Speech, audiobooks, voice notes" },
  { from: 96, label: "Podcast", hint: "Spoken word with music beds" },
  { from: 128, label: "Standard", hint: "Typical streaming quality" },
  { from: 192, label: "High", hint: "Music for good headphones" },
  { from: 256, label: "Max", hint: "Transparent for almost everyone" },
];

export function zoneFor(zones: Zone[], value: number): Zone {
  return [...zones].reverse().find((z) => value >= z.from) ?? zones[0];
}

/* ---------- Encoder detail (mirrors the backend mapping) ---------- */

const CRF_RANGE: Record<VideoCodec, [number, number]> = {
  h264: [18, 35],
  h265: [20, 38],
  av1: [22, 55],
  vp9: [24, 52],
};

export function videoCRF(codec: VideoCodec, quality: number): number {
  const [best, worst] = CRF_RANGE[codec];
  return Math.round(worst - ((quality - 1) * (worst - best)) / 99);
}

export function imageDetail(o: ImageOptions, sourceFormat?: string): string {
  const format = o.format === "original" ? sourceFormat : o.format;
  if (o.lossless && format !== "jpeg") return "lossless";
  switch (format) {
    case "png":
      return o.quality >= 100
        ? "lossless"
        : `${Math.min(256, Math.max(8, Math.round(2 ** (3 + (5 * o.quality) / 100))))} colours`;
    case "avif":
      return `CRF ${Math.round(50 - ((o.quality - 1) * 38) / 99)}`;
    default:
      return `q${o.quality}`;
  }
}

/* ---------- Labels ---------- */

export const VIDEO_CODECS: { value: VideoCodec; label: string; hint: string }[] = [
  { value: "h264", label: "H.264", hint: "Plays everywhere" },
  { value: "h265", label: "H.265", hint: "~40% smaller, newer devices" },
  { value: "av1", label: "AV1", hint: "Smallest, modern browsers" },
  { value: "vp9", label: "VP9", hint: "Web-friendly, WebM" },
];

export const CONTAINERS: { value: VideoContainer; label: string }[] = [
  { value: "mp4", label: "MP4" },
  { value: "webm", label: "WebM" },
  { value: "mkv", label: "MKV" },
  { value: "mov", label: "MOV" },
];

export const RESOLUTIONS = [
  { value: 0, label: "Original" },
  { value: 2160, label: "4K · 2160p" },
  { value: 1440, label: "1440p" },
  { value: 1080, label: "1080p" },
  { value: 720, label: "720p" },
  { value: 540, label: "540p" },
  { value: 480, label: "480p" },
  { value: 360, label: "360p" },
  { value: 240, label: "240p" },
];

export const FRAME_RATES = [0, 60, 50, 30, 25, 24, 15, 12].map((v) => ({
  value: v,
  label: v === 0 ? "Original" : `${v} fps`,
}));

export const AUDIO_FORMATS: {
  value: AudioOptions["format"];
  label: string;
  hint: string;
  lossy: boolean;
}[] = [
  { value: "mp3", label: "MP3", hint: "Universal", lossy: true },
  { value: "aac", label: "AAC", hint: "Apple, better than MP3", lossy: true },
  { value: "opus", label: "Opus", hint: "Best quality per byte", lossy: true },
  { value: "ogg", label: "OGG", hint: "Vorbis, open format", lossy: true },
  { value: "flac", label: "FLAC", hint: "Lossless, ~50% of WAV", lossy: false },
  { value: "wav", label: "WAV", hint: "Uncompressed", lossy: false },
  { value: "original", label: "Same", hint: "Keep the input format", lossy: true },
];

export const SAMPLE_RATES = [0, 48000, 44100, 32000, 24000, 22050, 16000, 11025, 8000].map(
  (v) => ({ value: v, label: v === 0 ? "Original" : `${(v / 1000).toFixed(v % 1000 ? 3 : 0)} kHz` })
);

export const IMAGE_FORMATS: { value: ImageOptions["format"]; label: string; hint: string }[] = [
  { value: "original", label: "Same", hint: "Keep the input format" },
  { value: "webp", label: "WebP", hint: "Small, supported everywhere" },
  { value: "avif", label: "AVIF", hint: "Smallest, modern browsers" },
  { value: "jpeg", label: "JPEG", hint: "Photos, maximum compatibility" },
  { value: "png", label: "PNG", hint: "Graphics, screenshots, transparency" },
];

export const IMAGE_SIZES = [0, 3840, 2560, 1920, 1280, 1024, 800, 512];

/* ---------- Presets ---------- */

export interface Preset<T extends FileType> {
  id: string;
  label: string;
  hint: string;
  apply: (base: OptionsByType[T]) => OptionsByType[T];
}

export const VIDEO_PRESETS: Preset<"video">[] = [
  {
    id: "balanced",
    label: "Balanced",
    hint: "H.264, good quality, plays everywhere",
    apply: (o) => ({ ...o, codec: "h264", container: "mp4", mode: "quality", quality: 60, resolution: 0, fps: 0, speed: "balanced" }),
  },
  {
    id: "discord",
    label: "Discord · 10 MB",
    hint: "Fits Discord's free upload limit",
    apply: (o) => ({ ...o, codec: "h264", container: "mp4", mode: "size", target_size_mb: 9.5, resolution: 720, fps: 30, audio_bitrate: 96 }),
  },
  {
    id: "whatsapp",
    label: "WhatsApp · 16 MB",
    hint: "Sends without WhatsApp re-compressing",
    apply: (o) => ({ ...o, codec: "h264", container: "mp4", mode: "size", target_size_mb: 15.5, resolution: 720, fps: 30, audio_bitrate: 96 }),
  },
  {
    id: "email",
    label: "Email · 25 MB",
    hint: "Under Gmail/Outlook attachment limits",
    apply: (o) => ({ ...o, codec: "h264", container: "mp4", mode: "size", target_size_mb: 24, resolution: 1080, audio_bitrate: 128 }),
  },
  {
    id: "web",
    label: "Web",
    hint: "1080p WebM/VP9 for <video> embeds",
    apply: (o) => ({ ...o, codec: "vp9", container: "webm", mode: "quality", quality: 55, resolution: 1080, fps: 0, audio_bitrate: 96 }),
  },
  {
    id: "smallest",
    label: "Smallest",
    hint: "AV1 at 720p, maximum savings",
    apply: (o) => ({ ...o, codec: "av1", container: "mp4", mode: "quality", quality: 40, resolution: 720, fps: 30, audio_bitrate: 64 }),
  },
  {
    id: "archive",
    label: "Archive",
    hint: "H.265, high quality, slow encode",
    apply: (o) => ({ ...o, codec: "h265", container: "mp4", mode: "quality", quality: 85, resolution: 0, fps: 0, speed: "slow", audio_bitrate: 192, strip_metadata: false }),
  },
];

export const AUDIO_PRESETS: Preset<"audio">[] = [
  {
    id: "music",
    label: "Music",
    hint: "AAC 192 kbps",
    apply: (o) => ({ ...o, format: "aac", bitrate: 192, channels: 0, sample_rate: 0, normalize: false, trim_silence: false }),
  },
  {
    id: "podcast",
    label: "Podcast / voice",
    hint: "Mono MP3 64 kbps, levelled, silence trimmed",
    apply: (o) => ({ ...o, format: "mp3", bitrate: 64, channels: 1, sample_rate: 44100, normalize: true, trim_silence: true }),
  },
  {
    id: "smallest",
    label: "Smallest",
    hint: "Opus 64 kbps, still sounds good",
    apply: (o) => ({ ...o, format: "opus", bitrate: 64, channels: 0, sample_rate: 0 }),
  },
  {
    id: "extract",
    label: "Extract MP3",
    hint: "Pull the soundtrack out of a video",
    apply: (o) => ({ ...o, format: "mp3", bitrate: 192, channels: 0, sample_rate: 0, normalize: false, trim_silence: false }),
  },
  {
    id: "lossless",
    label: "Lossless",
    hint: "FLAC, bit-perfect",
    apply: (o) => ({ ...o, format: "flac", bit_depth: 0, channels: 0, sample_rate: 0, normalize: false }),
  },
];

export const IMAGE_PRESETS: Preset<"image">[] = [
  {
    id: "web",
    label: "Web",
    hint: "WebP, max 1920 px",
    apply: (o) => ({ ...o, format: "webp", quality: 75, lossless: false, max_width: 1920, max_height: 1920 }),
  },
  {
    id: "smallest",
    label: "Smallest",
    hint: "AVIF, max 1920 px",
    apply: (o) => ({ ...o, format: "avif", quality: 55, lossless: false, max_width: 1920, max_height: 1920 }),
  },
  {
    id: "social",
    label: "Social",
    hint: "JPEG, max 2048 px",
    apply: (o) => ({ ...o, format: "jpeg", quality: 82, lossless: false, max_width: 2048, max_height: 2048 }),
  },
  {
    id: "email",
    label: "Email",
    hint: "JPEG, max 1280 px",
    apply: (o) => ({ ...o, format: "jpeg", quality: 70, lossless: false, max_width: 1280, max_height: 1280 }),
  },
  {
    id: "keep",
    label: "Same format",
    hint: "Recompress, keep size & format",
    apply: (o) => ({ ...o, format: "original", quality: 75, lossless: false, max_width: 0, max_height: 0 }),
  },
];

/* ---------- Capability-aware fix-ups ---------- */

/** Makes options valid for this server, e.g. if AV1 isn't installed. */
export function fitVideo(o: VideoOptions, caps?: Capabilities): VideoOptions {
  if (!caps) return o;
  const { video_codecs, video_containers } = caps.formats;
  let { codec, container } = o;
  if (!video_codecs.includes(codec)) codec = video_codecs[0] ?? "h264";
  if (!video_containers.includes(container) || !caps.container_codecs[container]?.includes(codec)) {
    container =
      video_containers.find((c) => caps.container_codecs[c]?.includes(codec)) ?? container;
  }
  return { ...o, codec, container };
}

export function fitAudio(o: AudioOptions, caps?: Capabilities): AudioOptions {
  if (!caps || o.format === "original" || caps.formats.audio_formats.includes(o.format)) return o;
  return { ...o, format: caps.formats.audio_formats[0] ?? "mp3" };
}

export function fitImage(o: ImageOptions, caps?: Capabilities): ImageOptions {
  if (!caps || o.format === "original" || caps.formats.image_formats.includes(o.format)) return o;
  return { ...o, format: "original" };
}

export const FALLBACK_DEFAULTS: OptionsByType = {
  video: {
    codec: "h264",
    container: "mp4",
    mode: "quality",
    quality: 60,
    target_size_mb: 10,
    speed: "balanced",
    resolution: 0,
    fps: 0,
    remove_audio: false,
    audio_bitrate: 128,
    trim: { start: 0, end: 0 },
    strip_metadata: true,
  },
  audio: {
    format: "mp3",
    bitrate: 128,
    channels: 0,
    sample_rate: 0,
    bit_depth: 0,
    normalize: false,
    trim_silence: false,
    trim: { start: 0, end: 0 },
    strip_metadata: false,
  },
  image: { format: "original", quality: 75, lossless: false, max_width: 0, max_height: 0 },
};
