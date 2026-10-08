import * as React from "react";
import { cn } from "@/lib/utils";
import { Switch } from "@/components/ui/switch";

interface FieldProps {
  label: string;
  hint?: React.ReactNode;
  htmlFor?: string;
  aside?: React.ReactNode;
  className?: string;
  children: React.ReactNode;
}

/** A labelled settings row: label (+ optional right-aligned aside) over a control. */
export function Field({ label, hint, htmlFor, aside, className, children }: FieldProps) {
  return (
    <div className={cn("space-y-1.5", className)}>
      <div className="flex items-baseline justify-between gap-2">
        <label htmlFor={htmlFor} className="text-xs font-medium">
          {label}
        </label>
        {aside && <span className="text-[11px] text-muted-foreground tabular-nums">{aside}</span>}
      </div>
      {children}
      {hint && <p className="text-[11px] leading-snug text-muted-foreground">{hint}</p>}
    </div>
  );
}

interface ToggleFieldProps {
  id: string;
  label: string;
  hint?: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  disabled?: boolean;
}

/** A label + description on the left with a switch on the right. */
export function ToggleField({ id, label, hint, checked, onChange, disabled }: ToggleFieldProps) {
  return (
    <div className={cn("flex items-start justify-between gap-4", disabled && "opacity-50")}>
      <label htmlFor={id} className="cursor-pointer space-y-0.5">
        <span className="block text-xs font-medium">{label}</span>
        {hint && <span className="block text-[11px] leading-snug text-muted-foreground">{hint}</span>}
      </label>
      <Switch id={id} checked={checked} onCheckedChange={onChange} disabled={disabled} className="mt-0.5" />
    </div>
  );
}

export function Section({
  title,
  children,
  className,
}: {
  title: string;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <section className={cn("space-y-4 border-t border-border pt-4", className)}>
      <h3 className="text-[11px] font-medium tracking-widest text-muted-foreground uppercase">{title}</h3>
      {children}
    </section>
  );
}
