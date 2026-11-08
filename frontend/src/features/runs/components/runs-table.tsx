"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";

import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ModalityBadge } from "@/features/agents/components/modality-badge";
import { formatDateTime, formatRelative } from "@/lib/format";
import type { RunSummary } from "@/lib/api/types";
import { cn } from "@/lib/utils";

import { formatDuration, formatLatency, isSlowLatency } from "../lib/format";
import { formatEndReason } from "../lib/status";
import { RunStatusBadge } from "./run-status-badge";
import { ScoreCell } from "./score";

const RIGHT_ALIGNED = new Set(["Score", "Duration", "p50"]);
const COLUMNS = ["Status", "Score", "Scenario", "Agent", "Persona", "Attempt", "Started", "Duration", "p50", "End reason"];

/** Stored duration, or the time between start and finish while metrics are missing. */
export function runDurationMs(run: Pick<RunSummary, "metrics" | "startedAt" | "finishedAt">): number | null {
  if (run.metrics) return run.metrics.durationMs;
  if (run.startedAt && run.finishedAt) return new Date(run.finishedAt).getTime() - new Date(run.startedAt).getTime();
  return null;
}

export function RunsTable({ slug, runs, loading }: { slug: string; runs: RunSummary[] | undefined; loading: boolean }) {
  const router = useRouter();
  return (
    <Card className="gap-0 overflow-hidden py-0">
      <Table>
        <TableHeader>
          <TableRow className="hover:bg-transparent">
            {COLUMNS.map((column) => (
              <TableHead key={column} className={cn("px-3 text-xs", RIGHT_ALIGNED.has(column) && "text-right")}>
                {column}
              </TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody aria-busy={loading || undefined}>
          {loading &&
            [0, 1, 2].map((row) => (
              <TableRow key={row}>
                {COLUMNS.map((column) => (
                  <TableCell key={column} className="px-3">
                    <Skeleton className="h-4 w-full max-w-24" />
                  </TableCell>
                ))}
              </TableRow>
            ))}
          {!loading &&
            runs?.map((run) => {
              const href = `/w/${slug}/runs/${run.id}`;
              const p50 = run.metrics?.latencyP50Ms ?? null;
              return (
                <TableRow
                  key={run.id}
                  className="cursor-pointer"
                  onClick={(event) => {
                    // Let modified clicks on the link open a new tab.
                    if ((event.target as HTMLElement).closest("a")) return;
                    router.push(href);
                  }}
                >
                  <TableCell className="px-3">
                    <RunStatusBadge status={run.status} />
                  </TableCell>
                  <TableCell className="px-3 text-right">
                    <ScoreCell score={run.score} passed={run.passed} judgementStatus={run.judgementStatus} />
                  </TableCell>
                  <TableCell className="max-w-56 px-3">
                    <Link href={href} className="block truncate font-medium outline-none hover:underline focus-visible:underline">
                      {run.scenarioName}
                    </Link>
                  </TableCell>
                  <TableCell className="max-w-48 px-3">
                    <span className="flex min-w-0 items-center gap-1.5">
                      <span className="truncate">{run.agentName}</span>
                      <ModalityBadge modality={run.modality} />
                    </span>
                  </TableCell>
                  <TableCell className="max-w-40 truncate px-3 text-muted-foreground">{run.personaName}</TableCell>
                  <TableCell className="px-3 text-muted-foreground tabular-nums">#{run.attempt}</TableCell>
                  <TableCell className="px-3 text-muted-foreground">
                    {run.startedAt ? (
                      <time dateTime={run.startedAt} title={formatDateTime(run.startedAt)}>
                        {formatRelative(run.startedAt)}
                      </time>
                    ) : (
                      "—"
                    )}
                  </TableCell>
                  <TableCell className="px-3 text-right tabular-nums">{formatDuration(runDurationMs(run))}</TableCell>
                  <TableCell className={cn("px-3 text-right tabular-nums", isSlowLatency(p50) && "text-destructive")}>
                    {formatLatency(p50)}
                  </TableCell>
                  <TableCell className="max-w-40 truncate px-3 text-muted-foreground" title={run.error ?? undefined}>
                    {formatEndReason(run.endReason)}
                  </TableCell>
                </TableRow>
              );
            })}
        </TableBody>
      </Table>
    </Card>
  );
}
