"use client";

import { BarChart3Icon, ClipboardListIcon, RotateCwIcon, XIcon } from "lucide-react";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useCallback, useMemo } from "react";

import { EmptyState } from "@/components/empty-state";
import { PageContainer } from "@/components/page-container";
import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { useAgents } from "@/features/agents/api/agents";
import { ScoreValue } from "@/features/runs/components/score";
import { formatLatency, formatTokens } from "@/features/runs/lib/format";
import { useScenarios } from "@/features/scenarios/api/scenarios";
import { SECTIONS, sectionHref } from "@/features/shell/lib/navigation";
import { useCurrentWorkspace } from "@/features/workspaces/session-context";
import { errorMessage } from "@/lib/api/errors";
import type { ReportOverview } from "@/lib/api/types";
import { cn } from "@/lib/utils";

import { useReportOverview } from "../api/reports";
import { fillTrendDays } from "../lib/chart";
import {
  NO_REPORT_FILTERS,
  REPORT_PERIODS,
  reportFiltersFromSearchParams,
  reportFiltersToQueryString,
  type ReportFilters,
} from "../lib/filters";
import { formatRate, totalRuns } from "../lib/format";
import { FailureTags, Kpi, ScenarioTable, WeakestCriteria } from "./report-sections";
import { TrendChart } from "./trend-chart";

const ALL = "all";

export function ReportsPage() {
  const workspace = useCurrentWorkspace();
  const slug = workspace.slug;
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const queryString = searchParams.toString();
  const filters = useMemo(() => reportFiltersFromSearchParams(new URLSearchParams(queryString)), [queryString]);

  const agents = useAgents(slug);
  const scenarios = useScenarios(slug);
  const report = useReportOverview(slug, filters);

  const setFilters = useCallback(
    (patch: Partial<ReportFilters>) => {
      const current = reportFiltersFromSearchParams(new URLSearchParams(window.location.search));
      const next = reportFiltersToQueryString({ ...current, ...patch });
      router.replace(next ? `${pathname}?${next}` : pathname, { scroll: false });
    },
    [router, pathname],
  );
  const filtered = filters.agentId !== null || filters.scenarioId !== null;

  let body;
  if (report.isPending) {
    body = <ReportSkeleton />;
  } else if (report.isError) {
    body = (
      <EmptyState
        icon={BarChart3Icon}
        title="Could not load the report"
        description={errorMessage(report.error)}
        action={
          <Button variant="outline" onClick={() => report.refetch()}>
            <RotateCwIcon />
            Try again
          </Button>
        }
      />
    );
  } else if (totalRuns(report.data.totals) === 0) {
    body = (
      <EmptyState
        icon={BarChart3Icon}
        title={filtered ? "No runs match" : "No runs in this period"}
        description="Reports fill in once runs are judged. Launch runs from Scenarios; each completed run is scored against its criteria."
        action={
          filtered ? (
            <Button variant="outline" onClick={() => setFilters({ agentId: null, scenarioId: null })}>
              Clear filters
            </Button>
          ) : (
            <Button variant="outline" asChild>
              <Link href={sectionHref(slug, SECTIONS.scenarios)}>
                <ClipboardListIcon />
                Open scenarios
              </Link>
            </Button>
          )
        }
      />
    );
  } else {
    body = <ReportBody report={report.data} slug={slug} />;
  }

  return (
    <PageContainer>
      <PageHeader title="Reports" description={SECTIONS.reports.description} />
      <div className="flex flex-wrap items-center gap-2">
        <div role="group" aria-label="Period" className="inline-flex rounded-lg border p-0.5">
          {REPORT_PERIODS.map((days) => (
            <Button
              key={days}
              type="button"
              size="sm"
              variant={filters.days === days ? "secondary" : "ghost"}
              aria-pressed={filters.days === days}
              className="h-7"
              onClick={() => setFilters({ days })}
            >
              {days} days
            </Button>
          ))}
        </div>
        <Select value={filters.agentId ?? ALL} onValueChange={(value) => setFilters({ agentId: value === ALL ? null : value })}>
          <SelectTrigger className="w-44" aria-label="Filter by agent">
            <SelectValue placeholder="All agents" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL}>All agents</SelectItem>
            {agents.data?.map((agent) => (
              <SelectItem key={agent.id} value={agent.id}>
                {agent.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Select
          value={filters.scenarioId ?? ALL}
          onValueChange={(value) => setFilters({ scenarioId: value === ALL ? null : value })}
        >
          <SelectTrigger className="w-52" aria-label="Filter by scenario">
            <SelectValue placeholder="All scenarios" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL}>All scenarios</SelectItem>
            {scenarios.data?.map((scenario) => (
              <SelectItem key={scenario.id} value={scenario.id}>
                {scenario.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {(filtered || filters.days !== NO_REPORT_FILTERS.days) && (
          <Button type="button" variant="ghost" size="sm" onClick={() => setFilters(NO_REPORT_FILTERS)}>
            <XIcon />
            Reset
          </Button>
        )}
      </div>
      <div className={cn("flex flex-col gap-6 transition-opacity", report.isPlaceholderData && "opacity-60")}>{body}</div>
    </PageContainer>
  );
}

function ReportSkeleton() {
  return (
    <div className="flex flex-col gap-6">
      <span className="sr-only" role="status">
        Loading report
      </span>
      <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
        {[0, 1, 2, 3].map((index) => (
          <Skeleton key={index} className="h-20 rounded-xl" />
        ))}
      </div>
      <Skeleton className="h-72 rounded-xl" />
    </div>
  );
}

function ReportBody({ report, slug }: { report: ReportOverview; slug: string }) {
  const runs = totalRuns(report.totals);
  const days = useMemo(() => fillTrendDays(report.trend, report.days), [report.trend, report.days]);
  const scenariosHref = sectionHref(slug, SECTIONS.scenarios);
  const runsHref = sectionHref(slug, SECTIONS.runs);
  const completed = report.totals.completed ?? 0;
  const judged = report.judged > 0;

  return (
    <>
      <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
        <Kpi label="Runs" value={runs} hint={`${completed} completed`} />
        <Kpi label="Judged" value={report.judged} hint={judged ? undefined : "No judged runs yet"} />
        <Kpi
          label="Average score"
          value={<ScoreValue score={report.avgScore === null ? null : Math.round(report.avgScore)} />}
          hint="out of 100"
        />
        <Kpi label="Pass rate" value={formatRate(report.passRate)} hint="of judged runs" />
        <Kpi label="Latency p50" value={formatLatency(report.latencyP50Ms)} hint="average per run" />
        <Kpi label="Latency p95" value={formatLatency(report.latencyP95Ms)} hint="average per run" />
        <Kpi label="LLM tokens" value={formatTokens(report.llmTokens)} className="col-span-2" />
      </div>

      <Card className="gap-4">
        <CardHeader>
          <CardTitle>Trend</CardTitle>
          <CardDescription>Runs and average score per day, last {report.days} days (UTC).</CardDescription>
        </CardHeader>
        <CardContent>
          <TrendChart days={days} />
        </CardContent>
      </Card>

      {!judged ? (
        <p className="rounded-lg border border-dashed bg-muted/20 px-4 py-6 text-center text-sm text-muted-foreground">
          Reports fill in once runs are judged. Completed runs are judged when their scenario has a judge connection and
          model.
        </p>
      ) : (
        <div className="grid items-start gap-6 lg:grid-cols-[minmax(0,1fr)_20rem]">
          <Card className="gap-4">
            <CardHeader>
              <CardTitle>Weakest criteria</CardTitle>
              <CardDescription>Lowest average score in the period.</CardDescription>
            </CardHeader>
            <CardContent>
              <WeakestCriteria items={report.weakestCriteria} scenarioHref={(id) => `${scenariosHref}/${id}`} />
            </CardContent>
          </Card>
          <Card className="gap-4">
            <CardHeader>
              <CardTitle>Failure tags</CardTitle>
              <CardDescription>Criteria carrying each tag.</CardDescription>
            </CardHeader>
            <CardContent>
              <FailureTags tags={report.failureTags} />
            </CardContent>
          </Card>
        </div>
      )}

      {report.scenarios.length > 0 && (
        <section className="flex flex-col gap-3" aria-labelledby="report-scenarios">
          <h2 id="report-scenarios" className="text-sm font-medium">
            Scenarios
          </h2>
          <ScenarioTable
            rows={report.scenarios}
            runsHref={(scenarioId) => `${runsHref}?scenarioId=${encodeURIComponent(scenarioId)}`}
          />
        </section>
      )}
    </>
  );
}
