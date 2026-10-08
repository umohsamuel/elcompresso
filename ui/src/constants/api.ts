export const API_BASE_URL = (
  process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"
).replace(/\/+$/, "");

export const API_ROUTES = {
  capabilities: "/api/v1/capabilities",
  jobs: "/api/v1/jobs",
  createJob: (type: string) => `/api/v1/jobs/${type}`,
  beginUpload: (type: string) => `/api/v1/uploads/${type}`,
  job: (id: string) => `/api/v1/jobs/${id}`,
} as const;

export const queryKeys = {
  capabilities: ["capabilities"] as const,
  jobs: (ids: string[]) => ["jobs", ...ids] as const,
} as const;
