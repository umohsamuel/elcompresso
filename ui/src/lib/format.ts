export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  const value = bytes / Math.pow(1024, i);
  return `${value.toFixed(i === 0 || value >= 100 ? 0 : 1)} ${units[i]}`;
}

export function formatDuration(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) return "–";
  const s = Math.round(seconds);
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = s % 60;
  const pad = (n: number) => n.toString().padStart(2, "0");
  return h > 0 ? `${h}:${pad(m)}:${pad(sec)}` : `${m}:${pad(sec)}`;
}

/**
 * Parses "90", "1:30", "1:02:03" or "1:30.5" into seconds.
 * Returns null for anything else; an empty string parses to 0.
 */
export function parseTime(input: string): number | null {
  const value = input.trim();
  if (value === "") return 0;
  if (!/^\d+(:\d{1,2}){0,2}(\.\d+)?$/.test(value)) return null;
  return value
    .split(":")
    .map(Number)
    .reduce((acc, part) => acc * 60 + part, 0);
}

export function formatTime(seconds: number): string {
  if (!seconds) return "";
  const whole = Math.floor(seconds);
  const frac = seconds - whole;
  const base = formatDuration(whole);
  return frac > 0 ? `${base}${frac.toFixed(2).slice(1)}` : base;
}
