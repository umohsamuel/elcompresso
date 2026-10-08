"use client";

import { useState } from "react";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { formatBytes } from "@/lib/format";

interface CompareDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  name: string;
  beforeUrl: string;
  afterUrl: string;
  beforeSize: number;
  afterSize: number;
}

/** Before/after image comparison with a draggable divider. */
export function CompareDialog({ open, onOpenChange, name, beforeUrl, afterUrl, beforeSize, afterSize }: CompareDialogProps) {
  const [split, setSplit] = useState(50);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-[min(96vw,1100px)] gap-3 rounded-none p-3 sm:max-w-[min(96vw,1100px)]">
        <DialogTitle className="truncate pr-8 text-sm">{name}</DialogTitle>
        <DialogDescription className="sr-only">Drag the slider to compare the original and compressed image.</DialogDescription>

        {/* Both images fill the same box so they line up even if one was resized. */}
        <div className="relative h-[70vh] w-full overflow-hidden bg-[repeating-conic-gradient(var(--muted)_0%_25%,transparent_0%_50%)] bg-[length:16px_16px] select-none">
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img src={afterUrl} alt="Compressed" className="absolute inset-0 h-full w-full object-contain" draggable={false} />
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img
            src={beforeUrl}
            alt="Original"
            className="absolute inset-0 h-full w-full object-contain"
            style={{ clipPath: `inset(0 ${100 - split}% 0 0)` }}
            draggable={false}
          />
          <div className="pointer-events-none absolute inset-y-0 w-px bg-white mix-blend-difference" style={{ left: `${split}%` }} />
          <span className="absolute top-2 left-2 bg-black/70 px-1.5 py-0.5 text-[10px] text-white">
            Original · {formatBytes(beforeSize)}
          </span>
          <span className="absolute top-2 right-2 bg-black/70 px-1.5 py-0.5 text-[10px] text-white">
            Compressed · {formatBytes(afterSize)}
          </span>
          <input
            type="range"
            min={0}
            max={100}
            step={0.1}
            value={split}
            onChange={(e) => setSplit(Number(e.target.value))}
            aria-label="Comparison position"
            className="absolute inset-0 h-full w-full cursor-ew-resize opacity-0"
          />
        </div>
      </DialogContent>
    </Dialog>
  );
}
