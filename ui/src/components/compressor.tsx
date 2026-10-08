"use client";

import { useEffect, useState } from "react";
import { ArrowCounterClockwiseIcon, FilmStripIcon, ImageIcon, MusicNotesIcon, XIcon } from "@phosphor-icons/react";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Button } from "@/components/ui/button";
import { FileDropZone } from "@/components/file-drop-zone";
import { JobList } from "@/components/job-list";
import { VideoSettings } from "@/components/settings/video-settings";
import { AudioSettings } from "@/components/settings/audio-settings";
import { ImageSettings } from "@/components/settings/image-settings";
import { chipClass } from "@/components/settings/video-settings";
import { useCapabilities } from "@/hooks/use-capabilities";
import { jobQueue, useJobQueue } from "@/lib/job-queue";
import { settingsStore, setSettings, resetSettings } from "@/lib/settings-store";
import { formatBytes } from "@/lib/format";
import {
  AUDIO_PRESETS,
  IMAGE_PRESETS,
  VIDEO_PRESETS,
  fitAudio,
  fitImage,
  fitVideo,
  sizeLimit,
  type Preset,
} from "@/lib/media";
import { createLocalStore } from "@/lib/local-store";
import type { Capabilities, FileType, OptionsByType } from "@/types/api";

const TABS: { value: FileType; label: string; icon: typeof ImageIcon }[] = [
  { value: "video", label: "Video", icon: FilmStripIcon },
  { value: "audio", label: "Audio", icon: MusicNotesIcon },
  { value: "image", label: "Image", icon: ImageIcon },
];

const tabStore = createLocalStore<FileType>("elcompresso.tab", "video");

export function Compressor() {
  const tab = tabStore.useValue();
  const settings = settingsStore.useValue();
  const items = useJobQueue();
  const { data: caps } = useCapabilities();
  const [staged, setStaged] = useState<Record<FileType, File[]>>({ video: [], audio: [], image: [] });

  useEffect(() => jobQueue.hydrate(), []);
  useEffect(() => {
    if (!caps) return;
    jobQueue.maxActive = Math.max(1, caps.limits.max_active_jobs || 3);
    jobQueue.directUpload = Boolean(caps.direct_upload);
  }, [caps]);

  const options = fit(tab, settings[tab], caps);
  const files = staged[tab];
  const totalSize = files.reduce((s, f) => s + f.size, 0);

  const addFiles = (incoming: File[]) =>
    setStaged((prev) => {
      const seen = new Set(prev[tab].map(key));
      return { ...prev, [tab]: [...prev[tab], ...incoming.filter((f) => !seen.has(key(f)))] };
    });

  const start = () => {
    if (!files.length) return;
    jobQueue.enqueue(tab, files, options);
    setStaged((prev) => ({ ...prev, [tab]: [] }));
  };

  // Phones: input → settings → staged files (sticky bar) → results.
  // Desktop: input, staged files and results on the left, settings on the right.
  return (
    <div className="flex flex-col gap-4 lg:grid lg:grid-cols-[minmax(0,1fr)_380px] lg:grid-rows-[auto_auto_1fr] lg:items-start lg:gap-x-6 lg:gap-y-4">
      {/* Input */}
      <div className="min-w-0 space-y-4 lg:col-start-1 lg:row-start-1">
        <Tabs value={tab} onValueChange={(v) => tabStore.set(v as FileType)}>
          <TabsList className="grid h-10! w-full grid-cols-3 sm:h-9!">
            {TABS.map(({ value, label, icon: Icon }) => (
              <TabsTrigger key={value} value={value} className="gap-1.5">
                <Icon /> {label}
                {staged[value].length > 0 && (
                  <span className="bg-foreground/15 px-1 text-[10px] tabular-nums">{staged[value].length}</span>
                )}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>

        <FileDropZone fileType={tab} maxBytes={sizeLimit(caps, tab)} onFiles={addFiles} />
      </div>

      {/* Settings */}
      <aside className="min-w-0 space-y-5 border border-border bg-card p-4 lg:sticky lg:top-6 lg:col-start-2 lg:row-span-3 lg:row-start-1 lg:max-h-[calc(100vh-3rem)] lg:overflow-y-auto">
        <div className="flex items-center justify-between">
          <h2 className="text-sm font-medium">{TABS.find((t) => t.value === tab)?.label} settings</h2>
          <Button
            size="xs"
            variant="ghost"
            onClick={() => resetSettings(tab)}
            title="Reset to defaults"
          >
            <ArrowCounterClockwiseIcon /> Reset
          </Button>
        </div>

        <Presets tab={tab} options={options} caps={caps} />

        {tab === "video" && <VideoSettings value={options as OptionsByType["video"]} onChange={(v) => setSettings("video", v)} caps={caps} />}
        {tab === "audio" && <AudioSettings value={options as OptionsByType["audio"]} onChange={(v) => setSettings("audio", v)} caps={caps} />}
        {tab === "image" && <ImageSettings value={options as OptionsByType["image"]} onChange={(v) => setSettings("image", v)} caps={caps} />}
      </aside>

      {/* Staged files. On phones this docks to the bottom of the screen so
          "Compress" stays reachable while scrolling through the settings. */}
      {files.length > 0 && (
        <div className="sticky bottom-0 z-20 -mx-4 min-w-0 space-y-3 border-t border-border bg-background/95 p-3 pb-[max(0.75rem,env(safe-area-inset-bottom))] backdrop-blur sm:mx-0 sm:border lg:static lg:col-start-1 lg:row-start-2 lg:bg-transparent lg:backdrop-blur-none">
          <div className="flex items-center justify-between text-xs">
            <span>
              <span className="font-medium">{files.length} ready</span>
              <span className="text-muted-foreground"> · {formatBytes(totalSize)}</span>
            </span>
            <Button size="xs" variant="ghost" className="max-sm:h-8" onClick={() => setStaged((p) => ({ ...p, [tab]: [] }))}>
              Clear
            </Button>
          </div>
          <ul className="max-h-24 space-y-1 overflow-y-auto sm:max-h-40">
            {files.map((f) => (
              <li key={key(f)} className="flex items-center gap-2 text-[11px]">
                <span className="min-w-0 flex-1 truncate" title={f.name}>{f.name}</span>
                <span className="text-muted-foreground tabular-nums">{formatBytes(f.size)}</span>
                <button
                  type="button"
                  aria-label={`Remove ${f.name}`}
                  className="-m-1.5 p-1.5 text-muted-foreground hover:text-foreground"
                  onClick={() => setStaged((p) => ({ ...p, [tab]: p[tab].filter((x) => x !== f) }))}
                >
                  <XIcon className="size-3.5 sm:size-3" />
                </button>
              </li>
            ))}
          </ul>
          <Button className="h-10 w-full text-sm sm:h-9" onClick={start}>
            Compress {files.length > 1 ? `${files.length} files` : "file"}
          </Button>
        </div>
      )}

      {/* Results */}
      <div className="min-w-0 lg:col-start-1 lg:row-start-3">
        <JobList items={items} />
      </div>
    </div>
  );
}

function Presets({ tab, options, caps }: { tab: FileType; options: OptionsByType[FileType]; caps?: Capabilities }) {
  const presets = { video: VIDEO_PRESETS, audio: AUDIO_PRESETS, image: IMAGE_PRESETS }[tab] as Preset<FileType>[];
  const current = JSON.stringify(options);

  return (
    <div className="space-y-1.5">
      <span className="text-[11px] font-medium tracking-widest text-muted-foreground uppercase">Presets</span>
      <div className="flex flex-wrap gap-1">
        {presets.map((p) => {
          const applied = fit(tab, p.apply(options), caps);
          return (
            <button
              key={p.id}
              type="button"
              title={p.hint}
              onClick={() => setSettings(tab, applied)}
              className={chipClass(JSON.stringify(applied) === current)}
            >
              {p.label}
            </button>
          );
        })}
      </div>
    </div>
  );
}

function fit<T extends FileType>(type: T, o: OptionsByType[T], caps?: Capabilities): OptionsByType[T] {
  switch (type) {
    case "video":
      return fitVideo(o as OptionsByType["video"], caps) as OptionsByType[T];
    case "audio":
      return fitAudio(o as OptionsByType["audio"], caps) as OptionsByType[T];
    default:
      return fitImage(o as OptionsByType["image"], caps) as OptionsByType[T];
  }
}

const key = (f: File) => `${f.name}:${f.size}:${f.lastModified}`;
