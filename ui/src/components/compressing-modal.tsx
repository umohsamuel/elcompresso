"use client";

import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import { useEffect, useState } from "react";

const MESSAGES = [
  "Reading file data...",
  "Encoding media stream...",
  "Optimizing bitrate...",
  "Compressing frames...",
  "Finalizing output...",
  "Uploading compressed file...",
  "Almost there...",
];

export function CompressingModal() {
  const [elapsed, setElapsed] = useState(0);
  const [messageIndex, setMessageIndex] = useState(0);

  useEffect(() => {
    const timer = setInterval(() => {
      setElapsed((prev) => prev + 1);
    }, 1000);

    return () => clearInterval(timer);
  }, []);

  useEffect(() => {
    const interval = setInterval(() => {
      setMessageIndex((prev) => (prev + 1) % MESSAGES.length);
    }, 8000);

    return () => clearInterval(interval);
  }, []);

  const minutes = Math.floor(elapsed / 60);
  const seconds = elapsed % 60;
  const timeDisplay =
    minutes > 0
      ? `${minutes}m ${seconds.toString().padStart(2, "0")}s`
      : `${seconds}s`;

  return (
    <Dialog open>
      <DialogContent
        className="sm:max-w-sm [&>button]:hidden rounded-lg"
        onPointerDownOutside={(e) => e.preventDefault()}
        onEscapeKeyDown={(e) => e.preventDefault()}
      >
        <div className="flex flex-col items-center gap-6 py-4">
          <PulseAnimation />

          <div className="text-center space-y-2">
            <DialogTitle className="text-base font-semibold">
              Compressing your file
            </DialogTitle>
            <DialogDescription className="text-sm text-muted-foreground">
              {MESSAGES[messageIndex]}
            </DialogDescription>
          </div>

          <div className="flex items-center gap-2 text-xs text-muted-foreground tabular-nums">
            <span>Elapsed: {timeDisplay}</span>
          </div>

          <p className="text-xs text-muted-foreground/60 text-center max-w-[240px]">
            Large files can take up to 2 minutes. Do not close this page.
          </p>
        </div>
      </DialogContent>
    </Dialog>
  );
}

function PulseAnimation() {
  return (
    <div className="relative flex items-center justify-center w-16 h-16">
      <div className="absolute w-16 h-16 rounded-full bg-foreground/5 animate-ping" />
      <div className="absolute w-12 h-12 rounded-full bg-foreground/10 animate-pulse" />
      <div className="w-6 h-6 rounded-full bg-foreground animate-pulse" />
    </div>
  );
}
