"use client";

import { useState } from "react";
import {
  ArrowClockwiseIcon,
  DownloadSimpleIcon,
  FilmStripIcon,
  ImageIcon,
  MusicNotesIcon,
  SplitHorizontalIcon,
  WarningIcon,
  XIcon,
} from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Progress } from "@/components/ui/progress";
import { CompareDialog } from "@/components/compare-dialog";
import { downloadUrl } from "@/lib/api/jobs";
import { formatBytes, formatDuration } from "@/lib/format";
import { isActive, jobQueue, type QueueItem } from "@/lib/job-queue";
import { cn } from "@/lib/utils";
import type { FileType } from "@/types/api";

const ICONS: Record<FileType, typeof ImageIcon> = {
  video: FilmStripIcon,
  audio: MusicNotesIcon,
  image: ImageIcon,
};

export function JobList({ items }: { items: QueueItem[] }) {
  if (!items.length) return null;

  const done = items.filter((it) => it.status === "done" && it.job?.result);
  const active = items.filter((it) => isActive(it.status)).length;
  const before = done.reduce((s, it) => s + it.size, 0);
  const after = done.reduce((s, it) => s + (it.job?.result?.size ?? 0), 0);
  const finished = items.length - active;

  const downloadAll = () => {
    done.forEach((it, i) => {
      setTimeout(() => triggerDownload(downloadUrl(it.job!.result!.download_url)), i * 400);
    });
  };

  return (
    <section aria-label="Files" className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="text-xs">
          <span className="font-medium">Files</span>
          <span className="text-muted-foreground">
            {" "}· {active > 0 ? `${active} in progress` : "all done"}
            {done.length > 0 && before > 0 && (
              <>
                {" "}· saved{" "}
                <span className="font-medium text-foreground">{formatBytes(Math.max(0, before - after))}</span>{" "}
                ({Math.round((1 - after / before) * 100)}%)
              </>
            )}
          </span>
        </div>
        <div className="flex gap-1.5">
          {done.length > 1 && (
            <Button size="xs" variant="outline" className="max-sm:h-8" onClick={downloadAll}>
              <DownloadSimpleIcon /> Download all ({done.length})
            </Button>
          )}
          {finished > 0 && (
            <Button size="xs" variant="ghost" className="max-sm:h-8" onClick={() => jobQueue.clearFinished()}>
              Clear finished
            </Button>
          )}
        </div>
      </div>

      <ul className="divide-y divide-border border border-border">
        {items.map((item) => (
          <JobRow key={item.id} item={item} />
        ))}
      </ul>
    </section>
  );
}

function JobRow({ item }: { item: QueueItem }) {
  const [comparing, setComparing] = useState(false);
  const Icon = ICONS[item.type];
  const result = item.job?.result;
  const { label, progress } = describe(item);

  return (
    <li className="space-y-2 p-3">
      <div className="flex items-start gap-2 sm:gap-3">
        <Icon className="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden />
        <div className="min-w-0 flex-1 space-y-0.5">
          <p className="truncate text-xs font-medium" title={item.name}>
            {result?.name ?? item.name}
          </p>
          <p className="text-[11px] text-muted-foreground tabular-nums">
            {formatBytes(item.size)}
            {result && (
              <>
                {" → "}
                <span className="text-foreground">{formatBytes(result.size)}</span>
                {describeInfo(result.info)}
              </>
            )}
          </p>
        </div>

        {result && (
          <span
            className={cn(
              "shrink-0 px-1.5 py-0.5 text-[11px] font-medium tabular-nums",
              result.larger ? "bg-destructive/15 text-destructive" : "bg-emerald-500/15 text-emerald-400"
            )}
          >
            {result.larger ? `+${Math.abs(result.saved_percent).toFixed(0)}%` : `-${result.saved_percent.toFixed(0)}%`}
          </span>
        )}

        <div className="flex shrink-0 gap-1">
          {result && item.type === "image" && item.previewUrl && (
            <Button size="icon-xs" variant="ghost" className="max-sm:size-8" aria-label="Compare before and after" title="Compare" onClick={() => setComparing(true)}>
              <SplitHorizontalIcon />
            </Button>
          )}
          {result && (
            <Button size="xs" className="max-sm:size-8 max-sm:px-0" asChild>
              <a href={downloadUrl(result.download_url)} download={result.name} aria-label={`Download ${result.name}`}>
                <DownloadSimpleIcon /> <span className="max-sm:sr-only">Download</span>
              </a>
            </Button>
          )}
          {(item.status === "failed" || item.status === "canceled") && item.file && (
            <Button size="icon-xs" variant="ghost" className="max-sm:size-8" aria-label="Retry" title="Retry" onClick={() => jobQueue.retry(item.id)}>
              <ArrowClockwiseIcon />
            </Button>
          )}
          <Button
            size="icon-xs"
            variant="ghost"
            className="max-sm:size-8"
            aria-label={isActive(item.status) ? "Cancel" : "Remove"}
            title={isActive(item.status) ? "Cancel" : "Remove"}
            onClick={() => (isActive(item.status) ? jobQueue.cancel(item.id) : jobQueue.remove(item.id))}
          >
            <XIcon />
          </Button>
        </div>
      </div>

      {(isActive(item.status) || item.status === "failed" || result?.larger) && (
        <div className="space-y-1.5 sm:pl-7">
          {isActive(item.status) && <Progress value={progress ?? 100} className={cn(progress === undefined && "animate-pulse")} />}
          <p
            className={cn(
              "flex items-center gap-1 text-[11px] tabular-nums",
              item.status === "failed" ? "text-destructive" : "text-muted-foreground"
            )}
            aria-live="polite"
          >
            {item.status === "failed" && <WarningIcon className="size-3 shrink-0" />}
            {item.status === "failed"
              ? item.error || "Compression failed"
              : result?.larger
                ? "Output is larger than the original - try a lower quality, a smaller resolution or a different format."
                : label}
          </p>
        </div>
      )}

      {result && item.previewUrl && (
        <CompareDialog
          open={comparing}
          onOpenChange={setComparing}
          name={result.name}
          beforeUrl={item.previewUrl}
          afterUrl={downloadUrl(result.download_url, true)}
          beforeSize={item.size}
          afterSize={result.size}
        />
      )}
    </li>
  );
}

function describe(item: QueueItem): { label: string; progress?: number } {
  const job = item.job;
  switch (item.status) {
    case "waiting":
      return { label: item.error ?? "Waiting to upload…", progress: 0 };
    case "uploading":
      return { label: `Uploading ${Math.round(item.upload * 100)}%`, progress: item.upload * 100 };
    case "queued":
      return {
        label: job?.queue_position && job.queue_position > 1 ? `Queued · #${job.queue_position} in line` : "Queued · starting soon",
        progress: 0,
      };
    case "processing": {
      const stage = job?.stage ? job.stage.charAt(0).toUpperCase() + job.stage.slice(1) : "Processing";
      // Images report no fine-grained progress.
      const pct = job?.progress ?? 0;
      return { label: `${stage} ${item.type === "image" ? "…" : `${pct.toFixed(0)}%`}`, progress: item.type === "image" ? undefined : pct };
    }
    default:
      return { label: "" };
  }
}

function describeInfo(info: { duration?: number; width?: number; height?: number; codec?: string }): string {
  const parts: string[] = [];
  if (info.width && info.height) parts.push(`${info.width}×${info.height}`);
  if (info.duration) parts.push(formatDuration(info.duration));
  if (info.codec) parts.push(info.codec.toUpperCase());
  return parts.length ? ` · ${parts.join(" · ")}` : "";
}

function triggerDownload(href: string) {
  const a = document.createElement("a");
  a.href = href;
  a.rel = "noopener";
  document.body.appendChild(a);
  a.click();
  a.remove();
}
