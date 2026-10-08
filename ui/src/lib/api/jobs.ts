import { AxiosError } from "axios";
import { apiClient } from "@/lib/api-client";
import { API_BASE_URL, API_ROUTES } from "@/constants/api";
import type {
  ApiEnvelope,
  ApiError,
  Capabilities,
  FileType,
  Job,
  OptionsByType,
} from "@/types/api";

export async function getCapabilities(): Promise<Capabilities> {
  const { data } = await apiClient.get<ApiEnvelope<Capabilities>>(
    API_ROUTES.capabilities
  );
  return data.data;
}

export async function createJob<T extends FileType>(
  type: T,
  file: File,
  options: OptionsByType[T],
  {
    signal,
    onUploadProgress,
  }: { signal?: AbortSignal; onUploadProgress?: (fraction: number) => void }
): Promise<Job> {
  // Options go first so the server can validate them before the file body.
  const form = new FormData();
  form.append("options", JSON.stringify(options));
  form.append("file", file);

  const { data } = await apiClient.post<ApiEnvelope<Job>>(
    API_ROUTES.createJob(type),
    form,
    {
      signal,
      timeout: 0, // large uploads can legitimately take minutes
      onUploadProgress: (e) => {
        if (e.total) onUploadProgress?.(e.loaded / e.total);
      },
    }
  );
  return data.data;
}

export async function getJobs(ids: string[]): Promise<Job[]> {
  const { data } = await apiClient.get<ApiEnvelope<Job[]>>(API_ROUTES.jobs, {
    params: { ids: ids.join(",") },
  });
  return data.data;
}

export async function cancelJob(id: string): Promise<void> {
  await apiClient.delete(API_ROUTES.job(id));
}

/** Resolves a server-relative download path against the API origin. */
export function downloadUrl(path: string, inline = false): string {
  const url = new URL(path, API_BASE_URL + "/");
  if (inline) url.searchParams.set("inline", "1");
  return url.toString();
}

export function errorMessage(err: unknown, fallback = "Something went wrong"): string {
  if (err instanceof AxiosError) {
    const apiErr = err.response?.data as ApiError | undefined;
    if (apiErr?.error) return apiErr.error;
    if (err.code === "ERR_NETWORK") return "Can't reach the server";
    return err.message;
  }
  return err instanceof Error ? err.message : fallback;
}
