export type FileType = "video" | "audio" | "image";

export interface CompressionResponse {
  message: string;
  data: {
    original_size: number;
    compressed_size: number;
    download_link: string;
  };
}

export interface UploadResponse {
  message: string;
  data: {
    url: string;
  };
}

export interface ApiError {
  message: string;
  error: string;
}

export interface CompressionFormData {
  file: File;
  quality: number;
}
