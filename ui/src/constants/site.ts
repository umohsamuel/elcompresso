// Public URL of this frontend (no trailing slash). Used for canonical URLs,
// the sitemap and absolute Open Graph image links.
export const SITE_URL = (
  process.env.NEXT_PUBLIC_SITE_URL || "https://elcompresso.vercel.app"
).replace(/\/+$/, "");

export const SITE = {
  name: "elcompresso",
  title: "elcompresso: free video, audio & image compressor by Samuel Umoh",
  description:
    "Compress, convert and trim video, audio and images in your browser. Hit an exact file size for Discord, WhatsApp or email, batch-process files and download the results. Built by Samuel Umoh.",
  keywords: [
    "video compressor",
    "compress video",
    "compress video for discord",
    "audio compressor",
    "image compressor",
    "compress mp4",
    "convert video",
    "h265",
    "av1",
    "webp",
    "avif",
    "ffmpeg online",
    "elcompresso",
    "Samuel Umoh",
  ],
  author: {
    name: "Samuel Umoh",
    url: "https://www.umohsg.com/",
  },
  // Matches --background in globals.css.
  themeColor: "#0a0a0a",
} as const;
