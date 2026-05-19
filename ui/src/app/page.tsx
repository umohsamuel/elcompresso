"use client";

import { CompressorTabs } from "@/components/compressor-tabs";

export default function Home() {
  return (
    <main className="flex-1 flex items-center justify-center px-4 py-12">
      <div className="w-full max-w-md space-y-8">
        <header className="text-center space-y-2">
          <h1 className="text-2xl font-bold tracking-tight">elcompresso</h1>
          <p className="text-sm text-muted-foreground">
            Compress video, audio, and image files
          </p>
        </header>

        <CompressorTabs />
      </div>
    </main>
  );
}
