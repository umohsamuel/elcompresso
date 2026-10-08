"use client";

import { QualitySlider } from "@/components/quality-slider";
import { Field, Section, ToggleField } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Segmented } from "@/components/ui/segmented";
import { chipClass } from "@/components/settings/video-settings";
import { IMAGE_FORMATS, IMAGE_SIZES, IMAGE_ZONES, imageDetail } from "@/lib/media";
import type { Capabilities, ImageOptions } from "@/types/api";

interface Props {
  value: ImageOptions;
  onChange: (value: ImageOptions) => void;
  caps?: Capabilities;
}

export function ImageSettings({ value: o, onChange, caps }: Props) {
  const set = (patch: Partial<ImageOptions>) => onChange({ ...o, ...patch });
  const available = (f: ImageOptions["format"]) =>
    f === "original" || !caps || caps.formats.image_formats.includes(f);
  const canLossless = o.format !== "jpeg";
  const lossless = canLossless && o.lossless;

  return (
    <div className="space-y-5">
      <Field label="Output format" hint={IMAGE_FORMATS.find((f) => f.value === o.format)?.hint}>
        <Segmented
          aria-label="Output format"
          value={o.format}
          onChange={(format) => set({ format })}
          options={IMAGE_FORMATS.map((f) => ({ ...f, disabled: !available(f.value) }))}
        />
      </Field>

      <QualitySlider
        value={o.quality}
        onChange={(quality) => set({ quality })}
        zones={IMAGE_ZONES}
        disabled={lossless}
        detail={o.format === "original" ? "varies by format" : imageDetail(o)}
      />

      {canLossless && (
        <ToggleField
          id="i-lossless"
          label="Lossless"
          hint={
            o.format === "png" || o.format === "original"
              ? "PNG: keep every colour. Off = palette quantization (like TinyPNG)."
              : "Pixel-perfect output. Larger, but no quality loss."
          }
          checked={o.lossless}
          onChange={(v) => set({ lossless: v })}
        />
      )}

      <Section title="Resize">
        <div className="grid grid-cols-2 gap-3">
          <Field label="Max width" htmlFor="i-w">
            <Input
              id="i-w"
              type="number"
              min={0}
              placeholder="Original"
              value={o.max_width || ""}
              onChange={(e) => set({ max_width: Math.max(0, Math.round(Number(e.target.value))) })}
            />
          </Field>
          <Field label="Max height" htmlFor="i-h">
            <Input
              id="i-h"
              type="number"
              min={0}
              placeholder="Original"
              value={o.max_height || ""}
              onChange={(e) => set({ max_height: Math.max(0, Math.round(Number(e.target.value))) })}
            />
          </Field>
        </div>
        <div className="flex flex-wrap gap-1">
          {IMAGE_SIZES.map((px) => (
            <button
              key={px}
              type="button"
              onClick={() => set({ max_width: px, max_height: px })}
              className={chipClass(o.max_width === px && o.max_height === px)}
            >
              {px === 0 ? "Original" : `${px}px`}
            </button>
          ))}
        </div>
        <p className="text-[11px] text-muted-foreground">
          Fits inside the box, keeps the aspect ratio and never upscales.
        </p>
      </Section>

      <p className="border-t border-border pt-4 text-[11px] leading-relaxed text-muted-foreground">
        EXIF data (GPS location, camera details) is always removed; photos are rotated upright first.
        {caps && ` Up to ${caps.limits.image_megapixels} megapixels.`}
      </p>
    </div>
  );
}
