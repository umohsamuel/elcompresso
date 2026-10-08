"use client";

import { ToggleGroup } from "radix-ui";
import { cn } from "@/lib/utils";

export interface SegmentedOption<T extends string | number> {
  value: T;
  label: string;
  hint?: string;
  disabled?: boolean;
}

interface SegmentedProps<T extends string | number> {
  value: T;
  onChange: (value: T) => void;
  options: SegmentedOption<T>[];
  disabled?: boolean;
  className?: string;
  "aria-label"?: string;
}

/** A single-choice button row. Values are stringified for Radix. */
export function Segmented<T extends string | number>({
  value,
  onChange,
  options,
  disabled,
  className,
  ...rest
}: SegmentedProps<T>) {
  return (
    <ToggleGroup.Root
      type="single"
      value={String(value)}
      onValueChange={(v) => {
        const match = options.find((o) => String(o.value) === v);
        if (match) onChange(match.value);
      }}
      disabled={disabled}
      aria-label={rest["aria-label"]}
      className={cn("grid auto-cols-fr grid-flow-col gap-px bg-border p-px", className)}
    >
      {options.map((o) => (
        <ToggleGroup.Item
          key={String(o.value)}
          value={String(o.value)}
          disabled={o.disabled}
          title={o.hint}
          className={cn(
            "h-9 min-w-0 truncate bg-background px-2 text-xs sm:h-7 text-muted-foreground transition-colors outline-none",
            "hover:bg-muted hover:text-foreground focus-visible:ring-1 focus-visible:ring-ring focus-visible:z-10",
            "data-[state=on]:bg-primary data-[state=on]:text-primary-foreground data-[state=on]:font-medium",
            "disabled:cursor-not-allowed disabled:opacity-40 disabled:hover:bg-background"
          )}
        >
          {o.label}
        </ToggleGroup.Item>
      ))}
    </ToggleGroup.Root>
  );
}
