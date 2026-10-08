"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { toast } from "sonner";
import { UploadSimpleIcon } from "@phosphor-icons/react";
import type { FileType } from "@/types/api";
import { ACCEPT, acceptsFile } from "@/lib/media";
import { formatBytes } from "@/lib/format";
import { cn } from "@/lib/utils";

interface FileDropZoneProps {
  fileType: FileType;
  maxBytes: number;
  onFiles: (files: File[]) => void;
  disabled?: boolean;
}

const HINTS: Record<FileType, string> = {
  video: "MP4, MOV, MKV, WebM, AVI and more",
  audio: "MP3, WAV, FLAC, AAC, OGG, M4A - or a video to extract its audio",
  image: "JPEG, PNG, WebP, AVIF, GIF, BMP, TIFF - or paste with Ctrl+V",
};

export function FileDropZone({ fileType, maxBytes, onFiles, disabled = false }: FileDropZoneProps) {
  const [isDragging, setIsDragging] = useState(false);
  const inputRef = useRef<HTMLInputElement | null>(null);
  const dragDepth = useRef(0);

  const handleFiles = useCallback(
    (list: FileList | File[]) => {
      const accepted: File[] = [];
      const rejected: string[] = [];
      for (const file of Array.from(list)) {
        if (!acceptsFile(fileType, file)) rejected.push(`${file.name}: not a ${fileType} file`);
        else if (file.size > maxBytes) rejected.push(`${file.name}: over the ${formatBytes(maxBytes)} limit`);
        else if (file.size === 0) rejected.push(`${file.name}: empty file`);
        else accepted.push(file);
      }
      if (rejected.length) {
        toast.error(`${rejected.length} file${rejected.length > 1 ? "s" : ""} skipped`, {
          description: rejected.slice(0, 4).join("\n") + (rejected.length > 4 ? "\n…" : ""),
        });
      }
      if (accepted.length) onFiles(accepted);
    },
    [fileType, maxBytes, onFiles]
  );

  // Paste images straight from the clipboard.
  useEffect(() => {
    if (fileType !== "image" || disabled) return;
    const onPaste = (e: ClipboardEvent) => {
      const target = e.target as HTMLElement | null;
      if (target?.closest("input, textarea, [contenteditable]")) return;
      const files = Array.from(e.clipboardData?.files ?? []);
      if (files.length) {
        e.preventDefault();
        handleFiles(files);
      }
    };
    window.addEventListener("paste", onPaste);
    return () => window.removeEventListener("paste", onPaste);
  }, [fileType, disabled, handleFiles]);

  return (
    <div
      role="button"
      tabIndex={disabled ? -1 : 0}
      aria-disabled={disabled}
      aria-label={`Add ${fileType} files`}
      onKeyDown={(e) => {
        if ((e.key === "Enter" || e.key === " ") && !disabled) {
          e.preventDefault();
          inputRef.current?.click();
        }
      }}
      onClick={() => !disabled && inputRef.current?.click()}
      onDragEnter={(e) => {
        e.preventDefault();
        dragDepth.current++;
        setIsDragging(true);
      }}
      onDragOver={(e) => e.preventDefault()}
      onDragLeave={() => {
        dragDepth.current = Math.max(0, dragDepth.current - 1);
        if (dragDepth.current === 0) setIsDragging(false);
      }}
      onDrop={(e) => {
        e.preventDefault();
        dragDepth.current = 0;
        setIsDragging(false);
        if (!disabled) handleFiles(e.dataTransfer.files);
      }}
      className={cn(
        "group flex min-h-[168px] cursor-pointer flex-col items-center justify-center gap-3 border border-dashed p-6 text-center transition-colors outline-none",
        "focus-visible:border-ring focus-visible:ring-1 focus-visible:ring-ring/50",
        isDragging ? "border-foreground bg-accent" : "border-foreground/20 hover:border-foreground/50 hover:bg-accent/40",
        disabled && "cursor-not-allowed opacity-50"
      )}
    >
      <input
        ref={inputRef}
        type="file"
        multiple
        accept={ACCEPT[fileType]}
        className="hidden"
        disabled={disabled}
        onChange={(e) => {
          if (e.target.files) handleFiles(e.target.files);
          e.target.value = ""; // allow picking the same file again
        }}
      />
      <UploadSimpleIcon
        className={cn("size-7 text-muted-foreground transition-transform", isDragging && "-translate-y-1 text-foreground")}
      />
      <div className="space-y-1">
        <p className="text-sm">
          {isDragging ? "Drop to add" : <>Drop {fileType} files here or <span className="underline underline-offset-4">browse</span></>}
        </p>
        <p className="text-[11px] text-muted-foreground">
          {HINTS[fileType]} · up to {formatBytes(maxBytes)} each
        </p>
      </div>
    </div>
  );
}
