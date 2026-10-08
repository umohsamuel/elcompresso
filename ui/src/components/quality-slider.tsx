"use client";

import { Slider as SliderPrimitive } from "radix-ui";
import { cn } from "@/lib/utils";
import { zoneFor, type Zone } from "@/lib/media";

interface QualitySliderProps {
  label?: string;
  value: number;
  onChange: (value: number) => void;
  zones: Zone[];
  /** Discrete stops (e.g. bitrates). When omitted the slider runs min..max. */
  values?: number[];
  min?: number;
  max?: number;
  /** Formats the headline value, e.g. v => `${v} kbps`. */
  format?: (value: number) => string;
  /** Technical detail shown under the slider, e.g. "CRF 24". */
  detail?: string;
  disabled?: boolean;
  lowLabel?: string;
  highLabel?: string;
}

export function QualitySlider({
  label = "Quality",
  value,
  onChange,
  zones,
  values,
  min = 1,
  max = 100,
  format = (v) => String(v),
  detail,
  disabled = false,
  lowLabel = "Smaller file",
  highLabel = "Better quality",
}: QualitySliderProps) {
  const discrete = values && values.length > 1;
  const index = discrete ? nearestIndex(values, value) : 0;
  const sliderMin = discrete ? 0 : min;
  const sliderMax = discrete ? values.length - 1 : max;
  const sliderValue = discrete ? index : value;

  const toValue = (pos: number) => (discrete ? values[pos] : pos);
  const percentOf = (v: number) =>
    discrete
      ? (nearestIndex(values, v) / (values.length - 1)) * 100
      : ((v - min) / (max - min)) * 100;

  const zone = zoneFor(zones, value);
  const segments = zones.map((z, i) => {
    const start = percentOf(z.from);
    const end = i < zones.length - 1 ? percentOf(zones[i + 1].from) : 100;
    const midValue = discrete
      ? values[Math.round(((start + end) / 2 / 100) * (values.length - 1))]
      : Math.round(min + ((start + end) / 2 / 100) * (max - min));
    return { zone: z, start, end, midValue };
  });

  return (
    <div className={cn("space-y-2.5", disabled && "pointer-events-none opacity-50")}>
      <div className="flex items-end justify-between gap-3">
        <div className="space-y-0.5">
          <span className="block text-xs font-medium">{label}</span>
          <span className="block text-[11px] text-muted-foreground">{zone.hint}</span>
        </div>
        <div className="text-right">
          <span className="block text-lg leading-none font-semibold tabular-nums">{format(value)}</span>
          <span className="mt-1 inline-block bg-primary px-1.5 py-px text-[10px] font-medium tracking-wide text-primary-foreground uppercase">
            {zone.label}
          </span>
        </div>
      </div>

      <SliderPrimitive.Root
        value={[sliderValue]}
        onValueChange={([v]) => onChange(toValue(v))}
        min={sliderMin}
        max={sliderMax}
        step={1}
        disabled={disabled}
        aria-label={label}
        className="relative flex h-5 w-full touch-none items-center select-none"
      >
        <SliderPrimitive.Track className="relative h-2 grow overflow-hidden bg-muted">
          {/* Zone boundaries */}
          {segments.slice(1).map((s) => (
            <span
              key={s.zone.label}
              aria-hidden
              className="absolute top-0 z-10 h-full w-px bg-background"
              style={{ left: `${s.start}%` }}
            />
          ))}
          <SliderPrimitive.Range className="absolute h-full bg-gradient-to-r from-foreground/35 to-foreground" />
        </SliderPrimitive.Track>
        <SliderPrimitive.Thumb
          aria-valuetext={`${format(value)}, ${zone.label}`}
          className="block h-5 w-2.5 border border-background bg-foreground shadow-sm ring-ring/50 transition-[box-shadow] outline-none hover:ring-2 focus-visible:ring-2"
        />
      </SliderPrimitive.Root>

      <div className="relative h-4" aria-hidden={disabled}>
        {segments.map((s) => (
          <button
            key={s.zone.label}
            type="button"
            tabIndex={-1}
            onClick={() => onChange(s.midValue)}
            className={cn(
              "absolute top-0 truncate text-center text-[10px] transition-colors hover:text-foreground",
              s.zone === zone ? "font-medium text-foreground" : "text-muted-foreground/70"
            )}
            style={{ left: `${s.start}%`, width: `${s.end - s.start}%` }}
          >
            {s.zone.label}
          </button>
        ))}
      </div>

      <div className="flex flex-wrap justify-between gap-x-3 text-[10px] text-muted-foreground/70">
        <span>← {lowLabel}</span>
        {detail && <span className="font-mono text-muted-foreground">{detail}</span>}
        <span>{highLabel} →</span>
      </div>
    </div>
  );
}

function nearestIndex(values: number[], v: number): number {
  let best = 0;
  for (let i = 1; i < values.length; i++) {
    if (Math.abs(values[i] - v) < Math.abs(values[best] - v)) best = i;
  }
  return best;
}
