"use client";

import { QualitySlider } from "@/components/quality-slider";
import { Field, Section, ToggleField } from "@/components/ui/field";
import { NativeSelect } from "@/components/ui/input";
import { Segmented } from "@/components/ui/segmented";
import { TrimField } from "@/components/settings/trim-field";
import { AUDIO_BITRATES, AUDIO_FORMATS, AUDIO_ZONES, SAMPLE_RATES } from "@/lib/media";
import type { AudioOptions, Capabilities } from "@/types/api";

interface Props {
  value: AudioOptions;
  onChange: (value: AudioOptions) => void;
  caps?: Capabilities;
}

export function AudioSettings({ value: o, onChange, caps }: Props) {
  const set = (patch: Partial<AudioOptions>) => onChange({ ...o, ...patch });
  const format = AUDIO_FORMATS.find((f) => f.value === o.format)!;
  const available = (f: AudioOptions["format"]) =>
    f === "original" || !caps || caps.formats.audio_formats.includes(f);
  const lossless = o.format === "flac" || o.format === "wav";

  return (
    <div className="space-y-5">
      <Field label="Output format" hint={format.hint}>
        <Segmented
          aria-label="Output format"
          value={o.format}
          onChange={(f) => set({ format: f })}
          options={AUDIO_FORMATS.map((f) => ({ ...f, disabled: !available(f.value) }))}
          className="grid-flow-row grid-cols-4 sm:grid-cols-7"
        />
      </Field>

      {lossless ? (
        <Field
          label="Bit depth"
          hint="16-bit is CD quality. Taking a 24-bit master down to 16-bit saves about a third."
        >
          <Segmented
            aria-label="Bit depth"
            value={o.bit_depth}
            onChange={(bit_depth) => set({ bit_depth })}
            options={[
              { value: 0, label: "Original" },
              { value: 16, label: "16-bit" },
              { value: 24, label: "24-bit" },
            ]}
          />
        </Field>
      ) : (
        <QualitySlider
          label="Bitrate"
          value={o.bitrate}
          onChange={(bitrate) => set({ bitrate })}
          values={AUDIO_BITRATES}
          zones={AUDIO_ZONES}
          format={(v) => `${v} kbps`}
          detail={o.format === "opus" ? "Opus VBR" : o.format === "original" ? "applies to lossy formats" : undefined}
        />
      )}

      <Section title="Channels & sample rate">
        <Field label="Channels" hint="Mono halves the size and is ideal for speech.">
          <Segmented
            aria-label="Channels"
            value={o.channels}
            onChange={(channels) => set({ channels })}
            options={[
              { value: 0, label: "Original" },
              { value: 2, label: "Stereo" },
              { value: 1, label: "Mono" },
            ]}
          />
        </Field>
        {o.format !== "opus" && (
          <Field label="Sample rate" htmlFor="a-sr" hint="44.1 kHz is plenty for music; 16-22 kHz is fine for voice.">
            <NativeSelect id="a-sr" value={o.sample_rate} onValueChange={(sample_rate) => set({ sample_rate })} options={SAMPLE_RATES} />
          </Field>
        )}
      </Section>

      <Section title="Processing">
        <ToggleField
          id="a-norm"
          label="Normalize loudness"
          hint="Even out volume to -16 LUFS (podcast/streaming standard)."
          checked={o.normalize}
          onChange={(normalize) => set({ normalize })}
        />
        <ToggleField
          id="a-silence"
          label="Trim silence"
          hint="Cut dead air from the start and end."
          checked={o.trim_silence}
          onChange={(trim_silence) => set({ trim_silence })}
        />
        <TrimField id="a-trim" value={o.trim} onChange={(trim) => set({ trim })} />
        <ToggleField
          id="a-meta"
          label="Strip metadata"
          hint="Remove tags like artist, album and comments."
          checked={o.strip_metadata}
          onChange={(strip_metadata) => set({ strip_metadata })}
        />
      </Section>
    </div>
  );
}
