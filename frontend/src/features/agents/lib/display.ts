import type { Modality, ProbeEvent } from "@/lib/api/types";

export const MODALITY_LABELS: Record<Modality, string> = {
  voice: "Voice",
  text: "Text",
};

/** Host (and port) of a transport URL; templates in the path do not matter. */
export function transportHost(url: string): string {
  try {
    return new URL(url).host || url;
  } catch {
    return url;
  }
}

/** Probe event offset, e.g. `1.23s`. */
export function formatProbeTime(atMs: number): string {
  return `${(atMs / 1000).toFixed(2)}s`;
}

/** Text color of each probe event kind in the (always dark) event log. */
export const PROBE_EVENT_COLORS: Record<ProbeEvent["kind"], string> = {
  connect: "text-sky-400",
  send: "text-violet-400",
  receive: "text-zinc-400",
  ready: "text-emerald-400",
  audio: "text-amber-400",
  transcript: "text-cyan-400",
  reply: "text-emerald-400",
  error: "text-red-400",
  close: "text-zinc-400",
  info: "text-zinc-500",
};

export const PROBE_DURATIONS = [5, 8, 15, 30] as const;
export const DEFAULT_PROBE_DURATION = 8;
export const DEFAULT_PROBE_MESSAGE = "Hello";
