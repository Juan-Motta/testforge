"use client";

import { ChevronRightIcon } from "lucide-react";

import type { ConnectionKind } from "@/lib/api/types";

import { CapabilityBadges, KindIcon } from "./capability-badges";

type KindPickerProps = {
  kinds: ConnectionKind[];
  onSelect: (kind: ConnectionKind) => void;
};

export function KindPicker({ kinds, onSelect }: KindPickerProps) {
  return (
    <ul className="grid gap-3 sm:grid-cols-2">
      {kinds.map((kind) => (
        <li key={kind.kind} className="flex">
          <button
            type="button"
            onClick={() => onSelect(kind)}
            className="group flex w-full flex-col gap-3 rounded-xl border bg-card p-4 text-left shadow-xs transition-colors outline-none hover:border-foreground/20 hover:bg-muted/40 focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50"
          >
            <span className="flex items-center gap-3">
              <KindIcon capabilities={kind.capabilities} />
              <span className="flex-1 text-sm font-medium">{kind.label}</span>
              <ChevronRightIcon
                aria-hidden
                className="size-4 text-muted-foreground transition-transform group-hover:translate-x-0.5"
              />
            </span>
            <span className="text-sm text-muted-foreground">{kind.description}</span>
            <CapabilityBadges capabilities={kind.capabilities} />
          </button>
        </li>
      ))}
    </ul>
  );
}
