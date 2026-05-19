"use client";

import { Slider } from "@/components/ui/slider";

interface QualitySliderProps {
  value: number;
  onChange: (value: number) => void;
  disabled?: boolean;
}

export function QualitySlider({
  value,
  onChange,
  disabled = false,
}: QualitySliderProps) {
  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <label className="text-sm font-medium">Quality</label>
        <span className="text-sm text-muted-foreground tabular-nums">
          {value}%
        </span>
      </div>
      <Slider
        value={[value]}
        onValueChange={([v]) => onChange(v)}
        min={1}
        max={100}
        step={1}
        disabled={disabled}
        className="w-full"
      />
      <div className="flex justify-between text-xs text-muted-foreground">
        <span>Smaller file</span>
        <span>Better quality</span>
      </div>
    </div>
  );
}
