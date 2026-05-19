export const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export const API_ROUTES = {
  compress: {
    video: "/api/v1/file-compress/video",
    audio: "/api/v1/file-compress/audio",
    image: "/api/v1/file-compress/image",
  },
  upload: "/api/v1/file-upload",
} as const;
