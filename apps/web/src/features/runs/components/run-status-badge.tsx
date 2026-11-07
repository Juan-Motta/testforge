import type { RunStatus } from "@/lib/api/types";
import { cn } from "@/lib/utils";

import { statusBadge, type StatusTone } from "../lib/status";

const TONES: Record<StatusTone, { badge: string; dot: string }> = {
  neutral: { badge: "bg-muted text-muted-foreground", dot: "bg-muted-foreground" },
  brand: { badge: "bg-brand/10 text-brand", dot: "bg-brand" },
  success: { badge: "bg-success/10 text-success", dot: "bg-success" },
  destructive: { badge: "bg-destructive/10 text-destructive", dot: "bg-destructive" },
  warning: { badge: "bg-warning/10 text-warning", dot: "bg-warning" },
};

export function RunStatusBadge({ status, className }: { status: RunStatus; className?: string }) {
  const info = statusBadge(status);
  const tone = TONES[info.tone];
  return (
    <span
      className={cn(
        "inline-flex h-5 w-fit shrink-0 items-center gap-1.5 rounded-4xl px-2 text-xs font-medium whitespace-nowrap",
        tone.badge,
        className,
      )}
    >
      <span aria-hidden className="relative flex size-1.5">
        {info.pulse && <span className={cn("absolute inline-flex size-full animate-ping rounded-full opacity-75", tone.dot)} />}
        <span className={cn("relative inline-flex size-1.5 rounded-full", tone.dot)} />
      </span>
      {info.label}
    </span>
  );
}
