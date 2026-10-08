"use client";

import { QualitySlider } from "@/components/quality-slider";
import { Field, Section, ToggleField } from "@/components/ui/field";
import { Input, NativeSelect } from "@/components/ui/input";
import { Segmented } from "@/components/ui/segmented";
import { TrimField } from "@/components/settings/trim-field";
import {
  CONTAINERS,
  FRAME_RATES,
  RESOLUTIONS,
  VIDEO_CODECS,
  VIDEO_ZONES,
  fitVideo,
  videoCRF,
} from "@/lib/media";
import type { Capabilities, VideoCodec, VideoContainer, VideoOptions } from "@/types/api";

interface Props {
  value: VideoOptions;
  onChange: (value: VideoOptions) => void;
  caps?: Capabilities;
}

const SIZE_CHIPS = [5, 8, 10, 16, 25, 50, 100];
const AUDIO_BITRATES = [48, 64, 96, 128, 160, 192, 256, 320];

export function VideoSettings({ value: o, onChange, caps }: Props) {
  const set = (patch: Partial<VideoOptions>) => onChange({ ...o, ...patch });

  const hasCodec = (c: VideoCodec) => !caps || caps.formats.video_codecs.includes(c);
  const fits = (container: VideoContainer, codec: VideoCodec) =>
    !caps || (caps.formats.video_containers.includes(container) && caps.container_codecs[container]?.includes(codec));

  return (
    <div className="space-y-5">
      <Field label="Compress by">
        <Segmented
          aria-label="Compression mode"
          value={o.mode}
          onChange={(mode) => set({ mode })}
          options={[
            { value: "quality", label: "Quality", hint: "Constant quality - size depends on the content" },
            { value: "size", label: "Target size", hint: "Hit a file size (two-pass encode)" },
          ]}
        />
      </Field>

      {o.mode === "quality" ? (
        <QualitySlider
          value={o.quality}
          onChange={(quality) => set({ quality })}
          zones={VIDEO_ZONES}
          detail={`${VIDEO_CODECS.find((c) => c.value === o.codec)?.label} CRF ${videoCRF(o.codec, o.quality)}`}
        />
      ) : (
        <Field
          label="Target size"
          htmlFor="v-target"
          aside="per file"
          hint="Lower resolution or removing audio helps very small targets look better."
        >
          <div className="flex items-center gap-2">
            <Input
              id="v-target"
              type="number"
              min={0.1}
              step={0.5}
              value={o.target_size_mb || ""}
              onChange={(e) => set({ target_size_mb: Number(e.target.value) })}
              className="w-24"
            />
            <span className="text-xs text-muted-foreground">MB</span>
          </div>
          <div className="flex flex-wrap gap-1">
            {SIZE_CHIPS.map((mb) => (
              <button
                key={mb}
                type="button"
                onClick={() => set({ target_size_mb: mb })}
                className={chipClass(o.target_size_mb === mb)}
              >
                {mb} MB
              </button>
            ))}
          </div>
        </Field>
      )}

      <Section title="Format">
        <Field label="Codec" hint={VIDEO_CODECS.find((c) => c.value === o.codec)?.hint}>
          <Segmented
            aria-label="Video codec"
            value={o.codec}
            onChange={(codec) => onChange(fitVideo({ ...o, codec }, caps))}
            options={VIDEO_CODECS.map((c) => ({ ...c, disabled: !hasCodec(c.value) }))}
          />
        </Field>
        <Field label="Container">
          <Segmented
            aria-label="Container"
            value={o.container}
            onChange={(container) => set({ container })}
            options={CONTAINERS.map((c) => ({
              ...c,
              disabled: !fits(c.value, o.codec),
              hint: fits(c.value, o.codec) ? undefined : `Can't hold ${o.codec.toUpperCase()}`,
            }))}
          />
        </Field>
        <Field label="Encoding speed" hint="Slower encodes produce smaller files at the same quality.">
          <Segmented
            aria-label="Encoding speed"
            value={o.speed}
            onChange={(speed) => set({ speed })}
            options={[
              { value: "fast", label: "Fast" },
              { value: "balanced", label: "Balanced" },
              { value: "slow", label: "Slow" },
            ]}
          />
        </Field>
      </Section>

      <Section title="Picture">
        <div className="grid grid-cols-2 gap-3">
          <Field label="Max resolution" htmlFor="v-res">
            <NativeSelect id="v-res" value={o.resolution} onValueChange={(resolution) => set({ resolution })} options={RESOLUTIONS} />
          </Field>
          <Field label="Max frame rate" htmlFor="v-fps">
            <NativeSelect id="v-fps" value={o.fps} onValueChange={(fps) => set({ fps })} options={FRAME_RATES} />
          </Field>
        </div>
        <p className="text-[11px] text-muted-foreground">Never upscales. Resolution applies to the short side, so vertical videos work too.</p>
      </Section>

      <Section title="Audio">
        <ToggleField
          id="v-mute"
          label="Remove audio"
          hint="Strip the soundtrack entirely (mute)."
          checked={o.remove_audio}
          onChange={(remove_audio) => set({ remove_audio })}
        />
        {!o.remove_audio && (
          <Field label="Audio bitrate" htmlFor="v-abr">
            <NativeSelect
              id="v-abr"
              value={o.audio_bitrate}
              onValueChange={(audio_bitrate) => set({ audio_bitrate })}
              options={AUDIO_BITRATES.map((b) => ({ value: b, label: `${b} kbps` }))}
            />
          </Field>
        )}
      </Section>

      <Section title="Edit">
        <TrimField id="v-trim" value={o.trim} onChange={(trim) => set({ trim })} />
        <ToggleField
          id="v-meta"
          label="Strip metadata"
          hint="Remove GPS location, device info and chapters."
          checked={o.strip_metadata}
          onChange={(strip_metadata) => set({ strip_metadata })}
        />
      </Section>
    </div>
  );
}

export function chipClass(active: boolean) {
  return [
    "h-6 px-2 text-[11px] border transition-colors",
    active
      ? "border-primary bg-primary text-primary-foreground"
      : "border-border text-muted-foreground hover:border-foreground/40 hover:text-foreground",
  ].join(" ");
}
