import { useSyncExternalStore } from "react";
import { AxiosError } from "axios";
import { cancelJob, createJob, errorMessage, getJobs } from "@/lib/api/jobs";
import type { FileType, Job, JobStatus, OptionsByType } from "@/types/api";

export type ItemStatus = "waiting" | "uploading" | JobStatus;

export interface QueueItem {
  id: string;
  type: FileType;
  name: string;
  size: number;
  /** Missing for items restored after a page reload. */
  file?: File;
  previewUrl?: string;
  options: OptionsByType[FileType];
  status: ItemStatus;
  upload: number; // 0..1
  job?: Job;
  error?: string;
  addedAt: number;
}

const STORAGE_KEY = "elcompresso.jobs.v1";
const POLL_MS = 1000;
const EMPTY: QueueItem[] = [];

export const isActive = (s: ItemStatus) => s === "waiting" || s === "uploading" || s === "queued" || s === "processing";
const onServer = (s: ItemStatus) => s === "queued" || s === "processing";

class JobQueue {
  private items: QueueItem[] = EMPTY;
  private listeners = new Set<() => void>();
  private controllers = new Map<string, AbortController>();
  private pollTimer: ReturnType<typeof setTimeout> | null = null;
  private retryTimer: ReturnType<typeof setTimeout> | null = null;
  private hydrated = false;
  maxActive = 3;
  /** Set from the server's capabilities: upload straight to S3. */
  directUpload = false;

  subscribe = (listener: () => void) => {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  };

  getSnapshot = () => this.items;
  getServerSnapshot = () => EMPTY;

  private commit(items: QueueItem[], save = true) {
    this.items = items;
    if (save) this.persist();
    this.listeners.forEach((l) => l());
  }

  private update(id: string, patch: Partial<QueueItem> | ((item: QueueItem) => Partial<QueueItem>), save = true) {
    this.commit(
      this.items.map((it) => (it.id === id ? { ...it, ...(typeof patch === "function" ? patch(it) : patch) } : it)),
      save
    );
  }

  /** Restore jobs from a previous visit so long encodes survive a reload. */
  hydrate() {
    if (this.hydrated || typeof window === "undefined") return;
    this.hydrated = true;
    try {
      const stored = JSON.parse(window.localStorage.getItem(STORAGE_KEY) ?? "[]") as QueueItem[];
      const now = Date.now();
      const restored = stored.filter(
        (it) => it.job && (!it.job.expires_at || Date.parse(it.job.expires_at) > now)
      );
      if (restored.length) {
        this.commit([...restored, ...this.items]);
        this.schedulePoll();
      }
    } catch {
      // ignore unreadable storage
    }
  }

  private persist() {
    try {
      // Only server-side jobs can be resumed; local files can't be stored.
      const keep = this.items
        .filter((it) => it.job && it.status !== "canceled")
        .map((it) => ({ ...it, file: undefined, previewUrl: undefined }));
      window.localStorage.setItem(STORAGE_KEY, JSON.stringify(keep));
    } catch {
      // ignore
    }
  }

  enqueue<T extends FileType>(type: T, files: File[], options: OptionsByType[T]) {
    const now = Date.now();
    const added: QueueItem[] = files.map((file, i) => ({
      id: `${now}-${i}-${Math.random().toString(36).slice(2, 8)}`,
      type,
      name: file.name,
      size: file.size,
      file,
      previewUrl: type === "image" ? URL.createObjectURL(file) : undefined,
      options: structuredClone(options),
      status: "waiting",
      upload: 0,
      addedAt: now,
    }));
    this.commit([...added, ...this.items]);
    this.pump();
  }

  /** Starts the next upload if we're under the server's concurrency limit. */
  private pump() {
    if (this.items.some((it) => it.status === "uploading")) return; // one upload at a time
    const active = this.items.filter((it) => it.status === "uploading" || onServer(it.status)).length;
    if (active >= this.maxActive) return;
    const next = [...this.items].reverse().find((it) => it.status === "waiting" && it.file);
    if (next) void this.upload(next);
  }

  private async upload(item: QueueItem) {
    const controller = new AbortController();
    this.controllers.set(item.id, controller);
    this.update(item.id, { status: "uploading", upload: 0, error: undefined });

    let lastEmit = 0;
    try {
      const job = await createJob(item.type, item.file!, item.options, {
        signal: controller.signal,
        direct: this.directUpload,
        onUploadProgress: (f) => {
          const now = performance.now();
          if (now - lastEmit > 100 || f === 1) {
            lastEmit = now;
            this.update(item.id, { upload: f }, false);
          }
        },
      });
      this.update(item.id, { status: job.status, job, upload: 1 });
      this.schedulePoll();
    } catch (err) {
      if (controller.signal.aborted) {
        this.update(item.id, { status: "canceled" });
      } else if (err instanceof AxiosError && (err.response?.status === 429 || err.response?.status === 503)) {
        // Server busy or we hit the per-client limit: wait and retry.
        const wait = Number(err.response.headers["retry-after"]) || 10;
        this.update(item.id, { status: "waiting", upload: 0, error: `${errorMessage(err)} - retrying in ${wait}s` });
        this.retryLater(wait * 1000);
      } else {
        this.update(item.id, { status: "failed", error: errorMessage(err, "Upload failed") });
      }
    } finally {
      this.controllers.delete(item.id);
      this.pump();
    }
  }

  private retryLater(ms: number) {
    if (this.retryTimer) return;
    this.retryTimer = setTimeout(() => {
      this.retryTimer = null;
      this.pump();
    }, ms);
  }

  private schedulePoll() {
    if (this.pollTimer) return;
    this.pollTimer = setTimeout(() => void this.poll(), POLL_MS);
  }

  private async poll() {
    this.pollTimer = null;
    const ids = this.items.filter((it) => it.job && onServer(it.status)).map((it) => it.job!.id);
    if (!ids.length) return;

    try {
      const jobs = new Map((await getJobs(ids)).map((j) => [j.id, j]));
      let changed = false;
      const next = this.items.map((it): QueueItem => {
        if (!it.job || !ids.includes(it.job.id)) return it;
        const job = jobs.get(it.job.id);
        if (!job) {
          changed = true;
          return { ...it, status: "failed", error: "The server no longer has this job (it may have restarted)." };
        }
        if (job.status === it.status && job.error === it.error && JSON.stringify(job) === JSON.stringify(it.job)) return it;
        changed = true;
        return { ...it, job, status: job.status, error: job.error };
      });
      if (changed) this.commit(next);
    } catch {
      // Network blip: keep polling.
    }
    this.pump();
    if (this.items.some((it) => onServer(it.status))) this.schedulePoll();
  }

  cancel(id: string) {
    const item = this.items.find((it) => it.id === id);
    if (!item) return;
    if (item.status === "uploading") {
      this.controllers.get(id)?.abort();
    } else if (item.job && onServer(item.status)) {
      void cancelJob(item.job.id).catch(() => undefined);
      this.update(id, { status: "canceled" });
    } else if (item.status === "waiting") {
      this.update(id, { status: "canceled" });
    }
    this.pump();
  }

  remove(id: string) {
    const item = this.items.find((it) => it.id === id);
    if (!item) return;
    if (isActive(item.status)) this.cancel(id);
    // Free server storage for finished results straight away.
    if (item.job && item.status === "done") void cancelJob(item.job.id).catch(() => undefined);
    if (item.previewUrl) URL.revokeObjectURL(item.previewUrl);
    this.commit(this.items.filter((it) => it.id !== id));
    this.pump();
  }

  retry(id: string) {
    const item = this.items.find((it) => it.id === id);
    if (!item?.file) return;
    this.update(id, { status: "waiting", upload: 0, job: undefined, error: undefined, addedAt: Date.now() });
    this.pump();
  }

  clearFinished() {
    const finished = this.items.filter((it) => !isActive(it.status));
    if (!finished.length) return;
    for (const item of finished) {
      if (item.job && item.status === "done") void cancelJob(item.job.id).catch(() => undefined);
      if (item.previewUrl) URL.revokeObjectURL(item.previewUrl);
    }
    this.commit(this.items.filter((it) => isActive(it.status)));
    this.pump();
  }
}

export const jobQueue = new JobQueue();

export function useJobQueue() {
  return useSyncExternalStore(jobQueue.subscribe, jobQueue.getSnapshot, jobQueue.getServerSnapshot);
}
