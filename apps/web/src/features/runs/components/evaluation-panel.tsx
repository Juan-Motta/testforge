"use client";

import { CircleAlertIcon, GaugeIcon, LightbulbIcon, RotateCwIcon, SparklesIcon, ThumbsUpIcon } from "lucide-react";
import Link from "next/link";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Spinner } from "@/components/ui/spinner";
import { errorMessage } from "@/lib/api/errors";
import type { CriterionResult, Judgement, JudgementResult, Run, Suggestion } from "@/lib/api/types";
import { cn } from "@/lib/utils";

import { useRejudgeRun } from "../api/judgement";
import {
  evidenceRefTarget,
  formatTag,
  groupSuggestions,
  isCappingTag,
  isJudgementPending,
  pendingJudgementText,
  unobservableReasonText,
} from "../lib/judgement";
import { isActiveStatus } from "../lib/status";
import { PassedBadge, ScoreMeter, ScoreValue, VerdictBadge } from "./score";

type TurnRef = { role: "user" | "agent" };

type EvaluationPanelProps = {
  slug: string;
  run: Run;
  turns: readonly TurnRef[];
  judgement: { data: Judgement | null | undefined; isPending: boolean; isError: boolean; error: unknown; refetch: () => void };
  canOperate: boolean;
  scenarioHref: string | null;
  onShowTurn: (index: number) => void;
};

export function EvaluationPanel({ slug, run, turns, judgement, canOperate, scenarioHref, onShowTurn }: EvaluationPanelProps) {
  const rejudge = useRejudgeRun(slug, run.id);
  const completed = run.status === "completed";
  const data = judgement.data;
  const pending = isJudgementPending(data?.status);
  const canRejudge = canOperate && completed && !pending && !judgement.isPending;

  function rejudgeRun() {
    rejudge.mutate(undefined, {
      onSuccess: () => toast.success("Judgement queued"),
      onError: (err) => toast.error(errorMessage(err)),
    });
  }

  let body;
  if (!completed) {
    body = (
      <p className="text-sm text-muted-foreground">
        {isActiveStatus(run.status)
          ? "The conversation is evaluated once the run completes."
          : "Only completed runs are evaluated."}
      </p>
    );
  } else if (judgement.isPending) {
    body = (
      <div className="flex flex-col gap-3">
        <span className="sr-only" role="status">
          Loading evaluation
        </span>
        <Skeleton className="h-16 w-full" />
        <Skeleton className="h-24 w-full" />
      </div>
    );
  } else if (judgement.isError) {
    body = (
      <div className="flex flex-col items-start gap-3 text-sm">
        <p className="text-muted-foreground">Could not load the evaluation: {errorMessage(judgement.error)}</p>
        <Button variant="outline" size="sm" onClick={() => judgement.refetch()}>
          <RotateCwIcon />
          Try again
        </Button>
      </div>
    );
  } else if (!data) {
    body = <p className="text-sm text-muted-foreground">This run has not been judged yet.</p>;
  } else if (pending) {
    body = (
      <p className="flex items-center gap-2 text-sm text-muted-foreground" role="status">
        <Spinner className="size-4" aria-hidden />
        {pendingJudgementText(data.status)}
      </p>
    );
  } else if (data.status === "skipped" || data.status === "failed" || !data.result) {
    body = <NotJudged judgement={data} scenarioHref={scenarioHref} />;
  } else {
    body = <JudgementView result={data.result} turns={turns} onShowTurn={onShowTurn} />;
  }

  return (
    <Card className="min-w-0 gap-4">
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <GaugeIcon aria-hidden className="size-4 text-brand" />
          Evaluation
        </CardTitle>
        {data?.status === "completed" && data.model && (
          <CardDescription>
            Judged by {data.model}
            {data.result ? ` · ${data.result.samples} sample${data.result.samples === 1 ? "" : "s"}` : ""}
          </CardDescription>
        )}
        {canRejudge && (
          <CardAction>
            <Button type="button" variant="outline" size="sm" onClick={rejudgeRun} disabled={rejudge.isPending}>
              {rejudge.isPending ? <Spinner /> : <RotateCwIcon />}
              {data ? "Re-judge" : "Judge"}
            </Button>
          </CardAction>
        )}
      </CardHeader>
      <CardContent>{body}</CardContent>
    </Card>
  );
}

function NotJudged({ judgement, scenarioHref }: { judgement: Judgement; scenarioHref: string | null }) {
  const skipped = judgement.status === "skipped";
  return (
    <div role="alert" className="flex items-start gap-2 rounded-lg border bg-muted/40 px-3 py-2.5 text-sm">
      <CircleAlertIcon aria-hidden className={cn("mt-0.5 size-4 shrink-0", skipped ? "text-muted-foreground" : "text-destructive")} />
      <div className="flex min-w-0 flex-col gap-1">
        <span className="font-medium">{skipped ? "Not judged" : "The judgement failed"}</span>
        {judgement.error && <span className="break-words text-muted-foreground">{judgement.error}</span>}
        <span className="text-muted-foreground">
          {skipped ? "Set a judge connection and model on the scenario, then re-judge." : "Check the scenario's judge connection, then re-judge."}
          {scenarioHref && (
            <>
              {" "}
              <Link href={scenarioHref} className="font-medium text-foreground underline-offset-4 hover:underline">
                Open the scenario
              </Link>
            </>
          )}
        </span>
      </div>
    </div>
  );
}

function JudgementView({
  result,
  turns,
  onShowTurn,
}: {
  result: JudgementResult;
  turns: readonly TurnRef[];
  onShowTurn: (index: number) => void;
}) {
  const groups = groupSuggestions(result.suggestions);
  return (
    <div className="flex flex-col gap-6">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-start">
        <div className="flex w-full shrink-0 flex-col gap-2 sm:w-40">
          <div className="flex items-baseline gap-1">
            <ScoreValue score={result.score} className="text-5xl leading-none tracking-tight" />
            <span className="text-sm text-muted-foreground">/ 100</span>
          </div>
          <ScoreMeter score={result.score} />
          <PassedBadge passed={result.passed} />
        </div>
        <div className="flex min-w-0 flex-col gap-2">
          {result.summary && <p className="text-sm leading-relaxed">{result.summary}</p>}
          {result.mustFailed && (
            <p className="text-sm text-red-700 dark:text-red-400">At least one “must” criterion failed.</p>
          )}
        </div>
      </div>

      <section className="flex flex-col gap-3" aria-labelledby="criteria-heading">
        <h3 id="criteria-heading" className="text-sm font-medium">
          Criteria
        </h3>
        <ul className="flex flex-col gap-3">
          {result.criteria.map((criterion, index) => (
            <CriterionItem key={criterion.criterionId || index} criterion={criterion} turns={turns} onShowTurn={onShowTurn} />
          ))}
        </ul>
      </section>

      {groups.length > 0 && (
        <section className="flex flex-col gap-3" aria-labelledby="suggestions-heading">
          <h3 id="suggestions-heading" className="flex items-center gap-2 text-sm font-medium">
            <LightbulbIcon aria-hidden className="size-4 text-muted-foreground" />
            Suggestions
          </h3>
          {groups.map((group) => (
            <div key={group.priority} className="flex flex-col gap-2">
              <span className="text-xs font-medium tracking-wide text-muted-foreground uppercase">
                {group.priority} priority
              </span>
              <ul className="flex flex-col gap-2">
                {group.items.map((suggestion, index) => (
                  <SuggestionItem key={index} suggestion={suggestion} turns={turns} onShowTurn={onShowTurn} />
                ))}
              </ul>
            </div>
          ))}
        </section>
      )}

      {result.strengths.length > 0 && (
        <section className="flex flex-col gap-2" aria-labelledby="strengths-heading">
          <h3 id="strengths-heading" className="flex items-center gap-2 text-sm font-medium">
            <ThumbsUpIcon aria-hidden className="size-4 text-muted-foreground" />
            Strengths
          </h3>
          <ul className="flex list-disc flex-col gap-1 pl-5 text-sm">
            {result.strengths.map((strength, index) => (
              <li key={index}>{strength}</li>
            ))}
          </ul>
        </section>
      )}
    </div>
  );
}

function CriterionItem({
  criterion,
  turns,
  onShowTurn,
}: {
  criterion: CriterionResult;
  turns: readonly TurnRef[];
  onShowTurn: (index: number) => void;
}) {
  const unobservable = unobservableReasonText(criterion.unobservableReason);
  return (
    <li className="flex flex-col gap-2.5 rounded-lg border px-3.5 py-3">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
        <p className="min-w-0 text-sm font-medium">{criterion.text}</p>
        <div className="flex shrink-0 items-center gap-2">
          <VerdictBadge verdict={criterion.verdict} />
          <ScoreValue score={criterion.score} className="w-8 text-right text-sm" />
        </div>
      </div>
      <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
        <span className={cn(criterion.kind === "must" && "font-medium text-foreground")}>
          {criterion.kind === "must" ? "Must" : "Should"}
        </span>
        <span>Source: {criterion.source}</span>
        <span className="tabular-nums">Weight {criterion.weight}</span>
        <span className="tabular-nums">Band {criterion.band ?? "—"}/5</span>
      </div>
      {criterion.failureTags.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {criterion.failureTags.map((tag) => (
            <TagChip key={tag} tag={tag} />
          ))}
        </div>
      )}
      {criterion.capped && (
        <p className="text-xs text-muted-foreground">
          Capped: a serious failure tag limited this criterion to band 2 at most.
        </p>
      )}
      {unobservable && <p className="text-sm text-muted-foreground">{unobservable}</p>}
      {criterion.reasoning && <p className="text-sm leading-relaxed text-muted-foreground">{criterion.reasoning}</p>}
      {criterion.evidence.length > 0 && (
        <ul className="flex flex-col gap-1.5">
          {criterion.evidence.map((evidence, index) => (
            <li key={index} className="flex items-start gap-2 text-sm">
              <EvidenceRef reference={evidence.ref} turns={turns} onShowTurn={onShowTurn} />
              <blockquote className="min-w-0 border-l-2 pl-2 break-words text-muted-foreground italic">
                “{evidence.quote}”
              </blockquote>
            </li>
          ))}
        </ul>
      )}
    </li>
  );
}

export function TagChip({ tag }: { tag: string }) {
  const capping = isCappingTag(tag);
  return (
    <span
      className={cn(
        "inline-flex h-5 items-center rounded-4xl px-2 text-[11px] font-medium",
        capping ? "bg-red-700 text-white dark:bg-red-400 dark:text-red-950" : "bg-muted text-muted-foreground",
      )}
      title={capping ? "Capping tag: limits the criterion to band 2" : undefined}
    >
      {formatTag(tag)}
    </span>
  );
}

const REF_KIND: Record<string, string> = { T: "Timing row", R: "Trace row" };

function EvidenceRef({
  reference,
  turns,
  onShowTurn,
}: {
  reference: string;
  turns: readonly TurnRef[];
  onShowTurn: (index: number) => void;
}) {
  const target = evidenceRefTarget(reference, turns);
  const chip = "shrink-0 rounded-md bg-muted px-1.5 py-px font-mono text-[11px] font-medium";
  if (target === null) {
    return (
      <span className={cn(chip, "text-muted-foreground")} title={REF_KIND[reference.trim().charAt(0).toUpperCase()]}>
        {reference}
      </span>
    );
  }
  const who = turns[target].role === "agent" ? "agent" : "user";
  return (
    <button
      type="button"
      onClick={() => onShowTurn(target)}
      className={cn(chip, "text-brand underline-offset-2 hover:underline focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none")}
      aria-label={`Show ${who} turn ${reference.trim().slice(1)} in the transcript`}
    >
      {reference}
    </button>
  );
}

const AREA_LABELS: Record<Suggestion["area"], string> = {
  prompt: "Prompt",
  tools: "Tools",
  flow: "Flow",
  latency: "Latency",
  voice: "Voice",
  other: "Other",
};

function SuggestionItem({
  suggestion,
  turns,
  onShowTurn,
}: {
  suggestion: Suggestion;
  turns: readonly TurnRef[];
  onShowTurn: (index: number) => void;
}) {
  return (
    <li className="flex flex-col gap-1.5 rounded-lg border px-3.5 py-3">
      <div className="flex flex-wrap items-center gap-2">
        <span className="inline-flex h-5 items-center gap-1 rounded-4xl border px-2 text-[11px] font-medium text-muted-foreground">
          <SparklesIcon aria-hidden className="size-3" />
          {AREA_LABELS[suggestion.area]}
        </span>
        <span className="text-sm font-medium">{suggestion.title}</span>
      </div>
      {suggestion.detail && <p className="text-sm text-muted-foreground">{suggestion.detail}</p>}
      {suggestion.evidenceRefs.length > 0 && (
        <div className="flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
          Evidence:
          {suggestion.evidenceRefs.map((reference) => (
            <EvidenceRef key={reference} reference={reference} turns={turns} onShowTurn={onShowTurn} />
          ))}
        </div>
      )}
    </li>
  );
}
