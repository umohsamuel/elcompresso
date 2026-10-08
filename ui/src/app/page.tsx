"use client";

import { Compressor } from "@/components/compressor";
import { useCapabilities } from "@/hooks/use-capabilities";
import { API_BASE_URL } from "@/constants/api";
import { cn } from "@/lib/utils";

export default function Home() {
  const { data: caps, isError, isPending, refetch } = useCapabilities();
  const ttlMinutes = caps ? Math.round(caps.limits.result_ttl_seconds / 60) : null;

  return (
    <main className="mx-auto w-full max-w-6xl flex-1 px-4 py-8 sm:py-12">
      <header className="mb-8 flex flex-wrap items-end justify-between gap-3">
        <div className="space-y-1">
          <h1 className="text-xl font-bold tracking-tight">elcompresso</h1>
          <p className="text-xs text-muted-foreground">
            Compress, convert and trim video, audio and images.
          </p>
        </div>
        <div className="flex items-center gap-2 text-[11px] text-muted-foreground" aria-live="polite">
          <span
            className={cn(
              "size-1.5 rounded-full",
              isPending ? "animate-pulse bg-muted-foreground" : isError ? "bg-destructive" : "bg-emerald-400"
            )}
          />
          {isPending ? "Connecting…" : isError ? "Server offline" : "Server online"}
        </div>
      </header>

      {isError && (
        <div role="alert" className="mb-6 flex flex-wrap items-center justify-between gap-2 border border-destructive/40 bg-destructive/10 p-3 text-xs text-destructive">
          <span>
            Can&apos;t reach the compression server at <code className="font-mono">{API_BASE_URL}</code>. Uploads will fail until it&apos;s back.
          </span>
          <button type="button" className="underline underline-offset-4" onClick={() => refetch()}>
            Retry
          </button>
        </div>
      )}

      <Compressor />

      <footer className="mt-12 border-t border-border pt-4 text-[11px] leading-relaxed text-muted-foreground">
        Files are processed on the server and deleted automatically
        {ttlMinutes ? ` ${ttlMinutes >= 60 ? `${Math.round(ttlMinutes / 60)}h` : `${ttlMinutes} min`} after compression` : ""}.
        Your settings are remembered in this browser.
      </footer>
    </main>
  );
}
