"use client";

import { useState } from "react";
import { Input } from "@/components/ui/input";
import { Field } from "@/components/ui/field";
import { formatTime, parseTime } from "@/lib/format";
import type { Trim } from "@/types/api";

interface TrimFieldProps {
  id: string;
  value: Trim;
  onChange: (trim: Trim) => void;
}

/** Start / end inputs accepting "90", "1:30" or "1:02:03.5". */
export function TrimField({ id, value, onChange }: TrimFieldProps) {
  const [start, setStart] = useState(formatTime(value.start));
  const [end, setEnd] = useState(formatTime(value.end));

  const s = parseTime(start);
  const e = parseTime(end);
  const invalidStart = s === null;
  const invalidEnd = e === null || (e > 0 && s !== null && e <= s);

  const commit = (nextStart: string, nextEnd: string) => {
    const ps = parseTime(nextStart);
    const pe = parseTime(nextEnd);
    if (ps === null || pe === null || (pe > 0 && pe <= ps)) return;
    onChange({ start: ps, end: pe });
  };

  return (
    <Field
      label="Trim"
      htmlFor={`${id}-start`}
      aside={value.start || value.end ? "active" : "full length"}
      hint={
        invalidEnd && !invalidStart && e !== null
          ? "End must be after start."
          : "Times as 90, 1:30 or 1:02:03. Leave end empty to keep going to the end."
      }
    >
      <div className="grid grid-cols-[1fr_auto_1fr] items-center gap-2">
        <Input
          id={`${id}-start`}
          inputMode="decimal"
          placeholder="0:00"
          value={start}
          aria-label="Trim start"
          aria-invalid={invalidStart}
          onChange={(ev) => {
            setStart(ev.target.value);
            commit(ev.target.value, end);
          }}
        />
        <span className="text-xs text-muted-foreground">to</span>
        <Input
          inputMode="decimal"
          placeholder="end"
          value={end}
          aria-label="Trim end"
          aria-invalid={invalidEnd}
          onChange={(ev) => {
            setEnd(ev.target.value);
            commit(start, ev.target.value);
          }}
        />
      </div>
    </Field>
  );
}
