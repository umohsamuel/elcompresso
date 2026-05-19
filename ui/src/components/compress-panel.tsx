"use client";

import { useState } from "react";
import { Button } from "@/components/ui/button";
import { FileDropZone } from "@/components/file-drop-zone";
import { QualitySlider } from "@/components/quality-slider";
import { CompressionResult } from "@/components/compression-result";
import { CompressingModal } from "@/components/compressing-modal";
import { useCompressMutation } from "@/hooks/use-compress-mutation";
import type { FileType } from "@/types/api";

interface CompressPanelProps {
  fileType: FileType;
}

export function CompressPanel({ fileType }: CompressPanelProps) {
  const [file, setFile] = useState<File | null>(null);
  const [quality, setQuality] = useState(50);

  const mutation = useCompressMutation({ fileType });

  const handleCompress = () => {
    if (!file) return;
    mutation.mutate({ file, quality });
  };

  const handleReset = () => {
    setFile(null);
    mutation.reset();
  };

  return (
    <div className="space-y-6">
      <FileDropZone
        fileType={fileType}
        onFileSelect={setFile}
        selectedFile={file}
        disabled={mutation.isPending}
      />

      <QualitySlider
        value={quality}
        onChange={setQuality}
        disabled={mutation.isPending}
      />

      <div className="flex gap-3">
        <Button
          onClick={handleCompress}
          disabled={!file || mutation.isPending}
          className="flex-1"
        >
          {mutation.isPending ? "Compressing..." : "Compress"}
        </Button>

        {(file || mutation.data) && (
          <Button variant="outline" onClick={handleReset}>
            Clear
          </Button>
        )}
      </div>

      {mutation.isError && (
        <p className="text-sm text-destructive">
          {mutation.error?.response?.data?.error ||
            "Compression failed. Try again."}
        </p>
      )}

      {mutation.data && <CompressionResult result={mutation.data.data} />}

      {mutation.isPending && <CompressingModal />}
    </div>
  );
}
