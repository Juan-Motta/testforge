import { ActivityIcon, ExternalLinkIcon } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import type { Judgement, TraceStats } from "@/lib/api/types";
import { cn } from "@/lib/utils";

import { isJudgementPending } from "../lib/judgement";
import { formatCost, formatLatency, formatTokens } from "../lib/format";

const STATUS_LABELS: Record<TraceStats["status"], string> = {
  available: "Available",
  unavailable: "Unavailable",
  not_configured: "Not configured",
  error: "Error",
};

function Stat({ label, value, alert }: { label: string; value: string; alert?: boolean }) {
  return (
    <div className="flex flex-col gap-0.5">
      <dt className="text-xs text-muted-foreground">{label}</dt>
      <dd className={cn("text-sm font-medium tabular-nums", alert && "text-destructive")}>{value}</dd>
    </div>
  );
}

/** Langfuse trace stats gathered while judging. */
export function TraceStatsCard({ judgement }: { judgement: Judgement | null | undefined }) {
  const traces = judgement?.traces ?? null;
  return (
    <Card className="gap-4">
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <ActivityIcon aria-hidden className="size-4 text-brand" />
          Traces
        </CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        {!traces ? (
          <p className="text-sm text-muted-foreground">
            {judgement && isJudgementPending(judgement.status)
              ? "Langfuse traces are fetched while the run is judged."
              : "Trace stats appear once the run is judged."}
          </p>
        ) : traces.status !== "available" ? (
          <div className="flex flex-col gap-1 text-sm">
            <span className="font-medium">{STATUS_LABELS[traces.status]}</span>
            {traces.message && <span className="text-muted-foreground">{traces.message}</span>}
          </div>
        ) : (
          <>
            <dl className="grid grid-cols-2 gap-x-4 gap-y-3">
              <Stat label="Traces" value={String(traces.traces)} />
              <Stat label="Observations" value={String(traces.observations)} />
              <Stat label="Errors" value={String(traces.errors)} alert={traces.errors > 0} />
              <Stat label="Tool calls" value={String(traces.toolCalls)} />
              <Stat label="Tokens" value={formatTokens(traces.totalTokens)} />
              <Stat label="Cost" value={formatCost(traces.totalCost)} />
              <Stat label="Slowest generation" value={formatLatency(traces.slowestGenerationMs)} />
              <Stat label="Time to first token" value={formatLatency(traces.timeToFirstTokenMs)} />
            </dl>
            {traces.models.length > 0 && (
              <div className="flex flex-col gap-1">
                <span className="text-xs text-muted-foreground">Models</span>
                <div className="flex flex-wrap gap-1.5">
                  {traces.models.map((model) => (
                    <span key={model} className="rounded-md bg-muted px-1.5 py-px font-mono text-[11px]">
                      {model}
                    </span>
                  ))}
                </div>
              </div>
            )}
            {traces.message && <p className="text-xs text-muted-foreground">{traces.message}</p>}
          </>
        )}
        {traces?.url && (
          <Button variant="outline" size="sm" asChild className="self-start">
            <a href={traces.url} target="_blank" rel="noreferrer">
              Open in Langfuse
              <ExternalLinkIcon data-icon="inline-end" />
            </a>
          </Button>
        )}
      </CardContent>
    </Card>
  );
}
