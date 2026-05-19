import { apiClient } from "@/lib/api-client";
import { API_ROUTES } from "@/constants/api";
import type { CompressionResponse, FileType } from "@/types/api";

export async function compressFile(
  fileType: FileType,
  file: File,
  quality: number
): Promise<CompressionResponse> {
  const formData = new FormData();
  formData.append("file", file);
  formData.append("quality", String(quality));

  const route = API_ROUTES.compress[fileType];
  const { data } = await apiClient.post<CompressionResponse>(route, formData, {
    headers: { "Content-Type": "multipart/form-data" },
  });

  return data;
}
