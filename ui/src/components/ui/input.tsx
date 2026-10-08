import * as React from "react"

import { cn } from "@/lib/utils"

const controlClass =
  "h-7 w-full min-w-0 rounded-none border border-input bg-transparent px-2 text-xs transition-colors outline-none placeholder:text-muted-foreground/60 focus-visible:border-ring focus-visible:ring-1 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50 aria-invalid:border-destructive dark:bg-input/30"

function Input({ className, type, ...props }: React.ComponentProps<"input">) {
  return (
    <input
      type={type}
      data-slot="input"
      className={cn(controlClass, "tabular-nums", className)}
      {...props}
    />
  )
}

interface NativeSelectProps<T extends string | number>
  extends Omit<React.ComponentProps<"select">, "value" | "onChange"> {
  value: T
  onValueChange: (value: T) => void
  options: { value: T; label: string; disabled?: boolean }[]
}

/** A styled native <select>: accessible and great on mobile. */
function NativeSelect<T extends string | number>({
  className,
  value,
  onValueChange,
  options,
  ...props
}: NativeSelectProps<T>) {
  return (
    <div className="relative">
      <select
        data-slot="select"
        value={String(value)}
        onChange={(e) => {
          const match = options.find((o) => String(o.value) === e.target.value)
          if (match) onValueChange(match.value)
        }}
        className={cn(controlClass, "cursor-pointer appearance-none pr-7 [&>option]:bg-popover", className)}
        {...props}
      >
        {options.map((o) => (
          <option key={String(o.value)} value={String(o.value)} disabled={o.disabled}>
            {o.label}
          </option>
        ))}
      </select>
      <svg
        aria-hidden
        viewBox="0 0 16 16"
        className="pointer-events-none absolute top-1/2 right-2 size-3 -translate-y-1/2 text-muted-foreground"
      >
        <path d="M4 6l4 4 4-4" fill="none" stroke="currentColor" strokeWidth="1.5" />
      </svg>
    </div>
  )
}

export { Input, NativeSelect }
