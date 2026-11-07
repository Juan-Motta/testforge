import {
  ActivityIcon,
  AudioLinesIcon,
  BrainCircuitIcon,
  MicIcon,
  PlugIcon,
  Volume2Icon,
  type LucideIcon,
} from "lucide-react";

import { Badge } from "@/components/ui/badge";
import type { Capability } from "@/lib/api/types";
import { cn } from "@/lib/utils";

import { CAPABILITY_LABELS } from "../lib/capabilities";

const CAPABILITY_ICONS: Record<Capability, LucideIcon> = {
  llm: BrainCircuitIcon,
  tts: Volume2Icon,
  stt: MicIcon,
  realtime: AudioLinesIcon,
  traces: ActivityIcon,
};

/** Neutral icon for a provider: its first capability, or a generic plug. */
export function KindIcon({ capabilities, className }: { capabilities: Capability[]; className?: string }) {
  const Icon = capabilities[0] ? CAPABILITY_ICONS[capabilities[0]] : PlugIcon;
  return (
    <span
      className={cn(
        "flex size-9 shrink-0 items-center justify-center rounded-lg border bg-background shadow-xs [&_svg]:size-4",
        className,
      )}
    >
      <Icon aria-hidden />
    </span>
  );
}

export function CapabilityBadges({ capabilities }: { capabilities: Capability[] }) {
  return (
    <ul className="flex flex-wrap gap-1.5" aria-label="Capabilities">
      {capabilities.map((capability) => {
        const Icon = CAPABILITY_ICONS[capability];
        return (
          <li key={capability}>
            <Badge variant="secondary" className="gap-1 font-normal">
              <Icon aria-hidden />
              {CAPABILITY_LABELS[capability]}
            </Badge>
          </li>
        );
      })}
    </ul>
  );
}
