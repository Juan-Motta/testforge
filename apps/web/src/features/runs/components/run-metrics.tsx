import type { Modality, RunMetrics as Metrics } from "@/lib/api/types";
import { cn } from "@/lib/utils";

import { formatDuration, formatLatency, isSlowLatency } from "../lib/format";

function Metric({ label, value, hint, alert }: { label: string; value: string; hint?: string; alert?: boolean }) {
  return (
    <div className="flex flex-col gap-1 rounded-lg border bg-card px-3 py-2.5">
      <span className="text-xs text-muted-foreground">{label}</span>
      <span className={cn("text-lg font-semibold tabular-nums", alert && "text-destructive")}>{value}</span>
      {hint && <span className="text-xs text-muted-foreground tabular-nums">{hint}</span>}
    </div>
  );
}

const number = new Intl.NumberFormat("en");

export function RunMetrics({ metrics, modality }: { metrics: Metrics | null; modality: Modality }) {
  if (!metrics) {
    return <p className="text-sm text-muted-foreground">Metrics appear once the conversation starts.</p>;
  }
  return (
    <div className="grid grid-cols-2 gap-3">
      <Metric label="Duration" value={formatDuration(metrics.durationMs)} />
      <Metric
        label="Turns"
        value={String(metrics.userTurns + metrics.agentTurns)}
        hint={`${metrics.userTurns} user · ${metrics.agentTurns} agent`}
      />
      <Metric
        label="Latency p50"
        value={formatLatency(metrics.latencyP50Ms)}
        alert={isSlowLatency(metrics.latencyP50Ms)}
      />
      <Metric
        label="Latency p95"
        value={formatLatency(metrics.latencyP95Ms)}
        alert={isSlowLatency(metrics.latencyP95Ms)}
      />
      {modality === "voice" && <Metric label="Agent audio" value={formatDuration(metrics.agentAudioMs)} />}
      <Metric
        label="LLM tokens"
        value={number.format(metrics.llmInputTokens + metrics.llmOutputTokens)}
        hint={`${number.format(metrics.llmInputTokens)} in · ${number.format(metrics.llmOutputTokens)} out`}
      />
    </div>
  );
}
