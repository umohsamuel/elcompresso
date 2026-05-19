export const mutationKeys = {
  compress: {
    video: ["compress", "video"] as const,
    audio: ["compress", "audio"] as const,
    image: ["compress", "image"] as const,
  },
  upload: ["upload"] as const,
} as const;
