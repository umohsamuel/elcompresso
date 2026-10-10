"use client";

import { ArrowUpRightIcon } from "@phosphor-icons/react";
import { Compressor } from "@/components/compressor";
import { useCapabilities } from "@/hooks/use-capabilities";
import { API_BASE_URL } from "@/constants/api";
import { SITE } from "@/constants/site";
import { cn } from "@/lib/utils";

export default function Home() {
  const { data: caps, isError, isPending, refetch } = useCapabilities();
  const ttlMinutes = caps ? Math.round(caps.limits.result_ttl_seconds / 60) : null;

  return (
    <main className="mx-auto w-full max-w-6xl flex-1 px-4 py-6 sm:py-12 lg:flex lg:h-svh lg:flex-col lg:overflow-hidden lg:py-8">
      <header className="mb-6 flex shrink-0 flex-wrap items-end justify-between gap-3 sm:mb-8 lg:mb-6">
        <div className="space-y-1">
          <h1 className="text-xl font-bold tracking-tight">elcompresso</h1>
          <p className="text-xs text-muted-foreground">
            Compress, convert and trim video, audio and images.{" "}
            <span className="whitespace-nowrap">
              by{" "}
              <a
                href={SITE.author.url}
                target="_blank"
                rel="author noopener"
                className="text-foreground underline-offset-4 hover:underline"
              >
                {SITE.author.name}
              </a>
            </span>
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
        <div role="alert" className="mb-6 flex shrink-0 flex-wrap items-center justify-between gap-2 border border-destructive/40 bg-destructive/10 p-3 text-xs text-destructive">
          <span>
            Can&apos;t reach the compression server at <code className="font-mono">{API_BASE_URL}</code>. Uploads will fail until it&apos;s back.
          </span>
          <button type="button" className="underline underline-offset-4" onClick={() => refetch()}>
            Retry
          </button>
        </div>
      )}

      <div className="lg:min-h-0 lg:flex-1">
        <Compressor />
      </div>

      <footer className="mt-12 flex shrink-0 flex-col gap-3 lg:mt-6 border-t border-border pt-4 text-[11px] leading-relaxed text-muted-foreground sm:flex-row sm:items-start sm:justify-between sm:gap-6">
        <p className="max-w-xl">
          Files are processed on the server and deleted automatically
          {ttlMinutes ? ` ${ttlMinutes >= 60 ? `${Math.round(ttlMinutes / 60)}h` : `${ttlMinutes} min`} after compression` : ""}.
          Your settings are remembered in this browser.
        </p>
        <div className="flex shrink-0 flex-wrap items-center gap-x-4 gap-y-1">
          <p>
            Built by{" "}
            <a
              href={SITE.author.url}
              target="_blank"
              rel="author noopener"
              className="font-medium text-foreground underline underline-offset-4 hover:text-foreground/80"
            >
              {SITE.author.name}
            </a>
          </p>
          {SITE.links.map((link) => (
            <a
              key={link.href}
              href={link.href}
              target="_blank"
              rel="noopener"
              className="inline-flex items-center gap-1 transition-colors hover:text-foreground"
            >
              {link.label}
              <ArrowUpRightIcon className="size-3" aria-hidden />
            </a>
          ))}
        </div>
      </footer>
    </main>
  );
}
