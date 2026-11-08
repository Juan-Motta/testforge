"use client";

import { ChevronDownIcon, ScrollTextIcon } from "lucide-react";

import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { cn } from "@/lib/utils";

import type { LiveLog, LogLevel } from "../lib/events";

const LEVEL_COLORS: Record<LogLevel, string> = {
  info: "text-sky-400",
  warn: "text-amber-400",
  error: "text-red-400",
};

export function RunLog({ logs, loading }: { logs: LiveLog[]; loading: boolean }) {
  const errors = logs.filter((log) => log.level === "error").length;
  return (
    <Collapsible className="rounded-xl border bg-card">
      <CollapsibleTrigger className="group flex w-full items-center gap-2 px-4 py-3 text-left text-sm font-medium outline-none focus-visible:ring-3 focus-visible:ring-ring/50">
        <ScrollTextIcon aria-hidden className="size-4 text-muted-foreground" />
        Run log
        <span className="text-xs font-normal text-muted-foreground tabular-nums">
          {logs.length} {logs.length === 1 ? "entry" : "entries"}
          {errors > 0 && <span className="text-destructive"> · {errors} errors</span>}
        </span>
        <ChevronDownIcon aria-hidden className="ml-auto size-4 text-muted-foreground transition-transform group-data-[state=open]:rotate-180" />
      </CollapsibleTrigger>
      <CollapsibleContent className="px-4 pb-4">
        <ol className="max-h-80 overflow-auto rounded-lg bg-zinc-950 p-3 font-mono text-[11px] leading-5 text-zinc-300 dark:ring-1 dark:ring-white/10">
          {logs.length === 0 && <li className="text-zinc-500">{loading ? "Loading…" : "No log entries."}</li>}
          {logs.map((log, index) => (
            <li key={log.id ?? `i${index}`} className="grid grid-cols-[3rem_minmax(0,1fr)] gap-2">
              <span className={cn(LEVEL_COLORS[log.level])}>{log.level}</span>
              <span className="break-words">{log.message}</span>
            </li>
          ))}
        </ol>
      </CollapsibleContent>
    </Collapsible>
  );
}
