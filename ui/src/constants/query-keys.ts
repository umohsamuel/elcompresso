export const queryKeys = {
  compression: {
    all: ["compression"] as const,
    video: ["compression", "video"] as const,
    audio: ["compression", "audio"] as const,
    image: ["compression", "image"] as const,
  },
} as const;
