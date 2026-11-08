import { AudioWaveformIcon } from "lucide-react";

import { cn } from "@/lib/utils";

export function LogoMark({ className }: { className?: string }) {
  return (
    <span
      aria-hidden
      className={cn(
        "flex size-8 shrink-0 items-center justify-center rounded-lg bg-brand text-brand-foreground shadow-sm",
        className,
      )}
    >
      <AudioWaveformIcon className="size-4" strokeWidth={2.25} />
    </span>
  );
}

export function Logo({ className }: { className?: string }) {
  return (
    <span className={cn("flex items-center gap-2.5", className)}>
      <LogoMark />
      <span className="text-base font-semibold tracking-tight">TestForge</span>
    </span>
  );
}
