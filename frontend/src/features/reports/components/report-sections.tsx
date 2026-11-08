import Link from "next/link";

import { Card } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ScoreValue } from "@/features/runs/components/score";
import { formatTag } from "@/features/runs/lib/judgement";
import { formatDateTime, formatRelative } from "@/lib/format";
import type { ReportScenarioRow, ReportWeakCriterion } from "@/lib/api/types";
import { cn } from "@/lib/utils";

import { formatRate, rankTags } from "../lib/format";

export function Kpi({ label, value, hint, className }: { label: string; value: React.ReactNode; hint?: string; className?: string }) {
  return (
    <Card className={cn("gap-1 px-4 py-3.5", className)}>
      <span className="text-xs text-muted-foreground">{label}</span>
      <span className="text-2xl font-semibold tracking-tight tabular-nums">{value}</span>
      {hint && <span className="text-xs text-muted-foreground tabular-nums">{hint}</span>}
    </Card>
  );
}

export function WeakestCriteria({ items, scenarioHref }: { items: ReportWeakCriterion[]; scenarioHref: (id: string) => string }) {
  if (items.length === 0) return <p className="text-sm text-muted-foreground">No judged criteria in this period.</p>;
  return (
    <ol className="flex flex-col divide-y">
      {items.map((item) => (
        <li key={`${item.scenarioId}-${item.criterionId}`} className="flex items-start gap-4 py-3 first:pt-0 last:pb-0">
          <div className="flex min-w-0 flex-1 flex-col gap-1">
            <p className="text-sm font-medium">{item.text}</p>
            <p className="truncate text-xs text-muted-foreground">
              {item.scenarioId ? (
                <Link href={scenarioHref(item.scenarioId)} className="hover:text-foreground hover:underline">
                  {item.scenarioName}
                </Link>
              ) : (
                item.scenarioName
              )}
            </p>
          </div>
          <div className="flex shrink-0 items-start gap-4 text-right text-xs text-muted-foreground">
            <div className="flex flex-col">
              <ScoreValue score={item.avgScore} className="text-base" />
              <span>avg</span>
            </div>
            <div className="flex flex-col">
              <span className="text-base font-semibold text-foreground tabular-nums">{formatRate(item.failRate)}</span>
              <span>fail rate</span>
            </div>
            <div className="flex flex-col">
              <span className="text-base font-semibold text-foreground tabular-nums">{item.judged}</span>
              <span>judged</span>
            </div>
          </div>
        </li>
      ))}
    </ol>
  );
}

export function FailureTags({ tags }: { tags: Record<string, number> }) {
  const ranked = rankTags(tags);
  if (ranked.length === 0) return <p className="text-sm text-muted-foreground">No failure tags in this period.</p>;
  return (
    <ol className="flex flex-col gap-2.5">
      {ranked.map((item) => (
        <li key={item.tag} className="flex flex-col gap-1">
          <div className="flex items-baseline justify-between gap-3 text-sm">
            <span className="truncate">{formatTag(item.tag)}</span>
            <span className="font-medium tabular-nums">{item.count}</span>
          </div>
          <div className="h-1.5 w-full rounded-full bg-muted" aria-hidden>
            <div className="h-full rounded-full bg-brand" style={{ width: `${Math.max(2, item.share * 100)}%` }} />
          </div>
        </li>
      ))}
    </ol>
  );
}

const SCENARIO_COLUMNS = ["Scenario", "Agent", "Runs", "Avg score", "Pass rate", "Last run"];
const NUMERIC = new Set(["Runs", "Avg score", "Pass rate"]);

export function ScenarioTable({ rows, runsHref }: { rows: ReportScenarioRow[]; runsHref: (scenarioId: string) => string }) {
  return (
    <Card className="gap-0 overflow-hidden py-0">
      <Table>
        <TableHeader>
          <TableRow className="hover:bg-transparent">
            {SCENARIO_COLUMNS.map((column) => (
              <TableHead key={column} className={cn("px-3 text-xs", NUMERIC.has(column) && "text-right")}>
                {column}
              </TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((row) => (
            <TableRow key={`${row.scenarioId ?? row.scenarioName}-${row.agentName}`}>
              <TableCell className="max-w-64 px-3">
                {row.scenarioId ? (
                  <Link
                    href={runsHref(row.scenarioId)}
                    className="block truncate font-medium hover:underline"
                    title="Show the runs of this scenario"
                  >
                    {row.scenarioName}
                  </Link>
                ) : (
                  <span className="block truncate font-medium">{row.scenarioName}</span>
                )}
              </TableCell>
              <TableCell className="max-w-48 truncate px-3 text-muted-foreground">{row.agentName}</TableCell>
              <TableCell className="px-3 text-right tabular-nums">{row.runs}</TableCell>
              <TableCell className="px-3 text-right">
                <ScoreValue score={row.avgScore === null ? null : Math.round(row.avgScore)} />
              </TableCell>
              <TableCell className="px-3 text-right tabular-nums">{formatRate(row.passRate)}</TableCell>
              <TableCell className="px-3 text-muted-foreground">
                <time dateTime={row.lastRunAt} title={formatDateTime(row.lastRunAt)}>
                  {formatRelative(row.lastRunAt)}
                </time>
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </Card>
  );
}
