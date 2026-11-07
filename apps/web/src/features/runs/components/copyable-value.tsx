"use client";

import { CopyIcon } from "lucide-react";

import { copyText } from "@/features/agents/components/copy-text";

export function CopyableValue({ label, value }: { label: string; value: string | null }) {
  return (
    <div className="flex min-w-0 flex-col gap-1">
      <span className="text-xs text-muted-foreground">{label}</span>
      {value ? (
        <button
          type="button"
          onClick={() => void copyText(value, `${label} copied`)}
          className="group flex min-w-0 items-center gap-2 rounded-md bg-muted px-2 py-1 text-left outline-none hover:bg-muted/70 focus-visible:ring-3 focus-visible:ring-ring/50"
          aria-label={`Copy ${label.toLowerCase()}`}
        >
          <code className="min-w-0 truncate font-mono text-xs">{value}</code>
          <CopyIcon aria-hidden className="ml-auto size-3.5 shrink-0 text-muted-foreground" />
        </button>
      ) : (
        <span className="px-2 py-1 text-xs text-muted-foreground">—</span>
      )}
    </div>
  );
}
