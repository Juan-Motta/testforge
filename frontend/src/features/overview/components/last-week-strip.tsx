"use client";

import { ArrowRightIcon } from "lucide-react";
import Link from "next/link";

import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { useReportOverview } from "@/features/reports/api/reports";
import { NO_REPORT_FILTERS } from "@/features/reports/lib/filters";
import { formatRate, totalRuns } from "@/features/reports/lib/format";
import { ScoreValue } from "@/features/runs/components/score";
import { SECTIONS, sectionHref } from "@/features/shell/lib/navigation";

function Figure({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-0.5">
      <span className="text-xs text-muted-foreground">{label}</span>
      <span className="text-xl font-semibold tracking-tight tabular-nums">{children}</span>
    </div>
  );
}

/** Compact "Last 7 days" summary, shown only once the workspace has judged runs. */
export function LastWeekStrip({ slug }: { slug: string }) {
  const report = useReportOverview(slug, NO_REPORT_FILTERS);
  if (!report.data || report.data.judged === 0) return null;
  const data = report.data;
  return (
    <Card className="flex-col gap-4 px-6 py-4 sm:flex-row sm:items-center">
      <span className="text-sm font-medium sm:w-28">Last 7 days</span>
      <div className="flex flex-1 flex-wrap gap-x-10 gap-y-3">
        <Figure label="Average score">
          <ScoreValue score={data.avgScore === null ? null : Math.round(data.avgScore)} />
        </Figure>
        <Figure label="Pass rate">{formatRate(data.passRate)}</Figure>
        <Figure label="Runs">{totalRuns(data.totals)}</Figure>
      </div>
      <Button variant="ghost" size="sm" asChild className="self-start sm:self-center">
        <Link href={sectionHref(slug, SECTIONS.reports)}>
          Open reports
          <ArrowRightIcon data-icon="inline-end" />
        </Link>
      </Button>
    </Card>
  );
}
