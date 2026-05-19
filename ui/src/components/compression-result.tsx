"use client";

import { Card, CardContent } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { formatBytes } from "@/components/file-drop-zone";
import type { CompressionResponse } from "@/types/api";

interface CompressionResultProps {
  result: CompressionResponse["data"];
}

export function CompressionResult({ result }: CompressionResultProps) {
  const ratio = (
    (1 - result.compressed_size / result.original_size) *
    100
  ).toFixed(1);

  return (
    <Card className="border-foreground/10">
      <CardContent className="p-5 space-y-4">
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-semibold">Compression Complete</h3>
          <Badge variant="secondary" className="font-mono text-xs">
            -{ratio}%
          </Badge>
        </div>

        <div className="grid grid-cols-2 gap-4 text-sm">
          <div>
            <p className="text-muted-foreground text-xs">Original</p>
            <p className="font-medium tabular-nums">
              {formatBytes(result.original_size)}
            </p>
          </div>
          <div>
            <p className="text-muted-foreground text-xs">Compressed</p>
            <p className="font-medium tabular-nums">
              {formatBytes(result.compressed_size)}
            </p>
          </div>
        </div>

        <Button asChild className="w-full" size="sm">
          <a
            href={result.download_link}
            target="_blank"
            rel="noopener noreferrer"
          >
            Download
          </a>
        </Button>
      </CardContent>
    </Card>
  );
}
