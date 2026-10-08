import { createLocalStore } from "@/lib/local-store";
import { FALLBACK_DEFAULTS } from "@/lib/media";
import type { FileType, OptionsByType } from "@/types/api";

/** Per-type compression settings, remembered between visits. */
export const settingsStore = createLocalStore<OptionsByType>(
  "elcompresso.settings.v1",
  FALLBACK_DEFAULTS,
  (stored) => {
    const s = (stored ?? {}) as Partial<OptionsByType>;
    // Merge so settings added in later versions get their defaults.
    return {
      video: { ...FALLBACK_DEFAULTS.video, ...s.video },
      audio: { ...FALLBACK_DEFAULTS.audio, ...s.audio },
      image: { ...FALLBACK_DEFAULTS.image, ...s.image },
    };
  }
);

export function setSettings<T extends FileType>(type: T, value: OptionsByType[T]) {
  settingsStore.set((prev) => ({ ...prev, [type]: value }));
}

export function resetSettings(type: FileType, defaults: OptionsByType = FALLBACK_DEFAULTS) {
  settingsStore.set((prev) => ({ ...prev, [type]: defaults[type] }));
}
