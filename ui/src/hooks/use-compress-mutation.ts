import { useMutation } from "@tanstack/react-query";
import { compressFile } from "@/lib/api/compress";
import { mutationKeys } from "@/constants/mutations";
import type { FileType, CompressionResponse, ApiError } from "@/types/api";
import { AxiosError } from "axios";

interface UseCompressMutationParams {
  fileType: FileType;
}

export function useCompressMutation({ fileType }: UseCompressMutationParams) {
  return useMutation<
    CompressionResponse,
    AxiosError<ApiError>,
    { file: File; quality: number }
  >({
    mutationKey: mutationKeys.compress[fileType],
    mutationFn: ({ file, quality }) => compressFile(fileType, file, quality),
  });
}
