"use client";

import { useCallback, useRef, useState } from "react";
import type { FileType } from "@/types/api";
import { cn } from "@/lib/utils";

const ACCEPT_MAP: Record<FileType, string> = {
  video: "video/mp4,video/mkv,video/avi,video/mov,video/webm,video/x-flv",
  audio: "audio/mpeg,audio/wav,audio/flac,audio/aac,audio/ogg,audio/mp4",
  image: "image/jpeg,image/png,image/webp",
};

const MAX_SIZE_MAP: Record<FileType, number> = {
  video: 500 * 1024 * 1024,
  audio: 100 * 1024 * 1024,
  image: 100 * 1024 * 1024,
};

interface FileDropZoneProps {
  fileType: FileType;
  onFileSelect: (file: File) => void;
  selectedFile: File | null;
  disabled?: boolean;
}

export function FileDropZone({
  fileType,
  onFileSelect,
  selectedFile,
  disabled = false,
}: FileDropZoneProps) {
  const [isDragging, setIsDragging] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const inputRef = useRef<HTMLInputElement | null>(null);

  const validateFile = useCallback(
    (file: File): boolean => {
      const maxSize = MAX_SIZE_MAP[fileType];
      if (file.size > maxSize) {
        setError(
          `File too large. Max ${Math.round(maxSize / (1024 * 1024))}MB.`
        );
        return false;
      }
      setError(null);
      return true;
    },
    [fileType]
  );

  const handleFile = useCallback(
    (file: File) => {
      if (validateFile(file)) {
        onFileSelect(file);
      }
    },
    [validateFile, onFileSelect]
  );

  const handleDrop = useCallback(
    (e: React.DragEvent<HTMLDivElement>) => {
      e.preventDefault();
      setIsDragging(false);
      const file = e.dataTransfer.files[0];
      if (file) handleFile(file);
    },
    [handleFile]
  );

  const handleChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const file = e.target.files?.[0];
      if (file) handleFile(file);
    },
    [handleFile]
  );

  return (
    <div className="space-y-2">
      <div
        onDrop={handleDrop}
        onDragOver={(e) => {
          e.preventDefault();
          setIsDragging(true);
        }}
        onDragEnter={() => setIsDragging(true)}
        onDragLeave={() => setIsDragging(false)}
        onClick={() => !disabled && inputRef.current?.click()}
        className={cn(
          "border-2 border-dashed rounded-xl p-8 text-center transition-all cursor-pointer",
          "flex flex-col items-center justify-center gap-3 min-h-[180px]",
          isDragging && "border-foreground bg-accent",
          !isDragging &&
            "border-muted-foreground/30 hover:border-foreground/50",
          disabled && "opacity-50 cursor-not-allowed",
          selectedFile && "border-foreground/60 bg-accent/50"
        )}
      >
        <input
          ref={inputRef}
          type="file"
          accept={ACCEPT_MAP[fileType]}
          onChange={handleChange}
          className="hidden"
          disabled={disabled}
        />

        {selectedFile ? (
          <SelectedFileInfo file={selectedFile} />
        ) : (
          <DropZonePlaceholder fileType={fileType} />
        )}
      </div>

      {error && <p className="text-sm text-destructive font-medium">{error}</p>}
    </div>
  );
}

function DropZonePlaceholder({ fileType }: { fileType: FileType }) {
  return (
    <>
      <p className="text-sm text-muted-foreground">
        Drop your {fileType} file here or click to browse
      </p>
      <p className="text-xs text-muted-foreground/60">
        Max {fileType === "video" ? "500" : "100"}MB
      </p>
    </>
  );
}

function SelectedFileInfo({ file }: { file: File }) {
  const sizeLabel = formatBytes(file.size);
  return (
    <div className="space-y-1">
      <p className="text-sm font-medium truncate max-w-[280px]">{file.name}</p>
      <p className="text-xs text-muted-foreground">{sizeLabel}</p>
    </div>
  );
}

export function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const units = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
}
