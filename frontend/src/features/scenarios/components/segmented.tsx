"use client";

import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

export type SegmentedOption<T extends string> = {
  value: T;
  label: string;
  /** When set, the option is disabled and this explains why (in a tooltip). */
  disabledReason?: string;
};

type SegmentedProps<T extends string> = {
  value: T;
  onChange: (value: T) => void;
  options: SegmentedOption<T>[];
  label: string;
  id?: string;
  disabled?: boolean;
  invalid?: boolean;
  className?: string;
};

/** Small radio group drawn as a segmented control. */
export function Segmented<T extends string>({
  value,
  onChange,
  options,
  label,
  id,
  disabled,
  invalid,
  className,
}: SegmentedProps<T>) {
  return (
    <div
      id={id}
      role="radiogroup"
      aria-label={label}
      aria-invalid={invalid || undefined}
      className={cn(
        "inline-flex w-fit! self-start rounded-lg border bg-muted/40 p-0.5 aria-invalid:border-destructive",
        className,
      )}
    >
      {options.map((option) => {
        const active = option.value === value;
        const unavailable = option.disabledReason !== undefined;
        const button = (
          <button
            type="button"
            role="radio"
            aria-checked={active}
            disabled={disabled || unavailable}
            onClick={() => onChange(option.value)}
            className={cn(
              "inline-flex h-6 items-center rounded-md px-2 text-xs transition-colors outline-none focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50",
              active ? "bg-background font-medium shadow-xs" : "text-muted-foreground enabled:hover:text-foreground",
            )}
          >
            {option.label}
          </button>
        );
        if (!unavailable) return <span key={option.value}>{button}</span>;
        return (
          <Tooltip key={option.value}>
            {/* Disabled buttons do not fire pointer events, so the wrapper carries the tooltip. */}
            <TooltipTrigger asChild>
              <span tabIndex={disabled ? undefined : 0} className="rounded-md outline-none focus-visible:ring-3 focus-visible:ring-ring/50">
                {button}
              </span>
            </TooltipTrigger>
            <TooltipContent>{option.disabledReason}</TooltipContent>
          </Tooltip>
        );
      })}
    </div>
  );
}
