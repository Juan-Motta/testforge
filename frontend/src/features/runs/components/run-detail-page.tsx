"use client";

import { useQueryClient } from "@tanstack/react-query";
import {
  ArrowLeftIcon,
  AudioLinesIcon,
  BotIcon,
  CircleAlertIcon,
  PlayIcon,
  RotateCcwIcon,
  RotateCwIcon,
  SquareIcon,
  UserRoundIcon,
} from "lucide-react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { toast } from "sonner";

import { EmptyState } from "@/components/empty-state";
import { PageContainer } from "@/components/page-container";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Spinner } from "@/components/ui/spinner";
import { ModalityBadge } from "@/features/agents/components/modality-badge";
import { SECTIONS, sectionHref } from "@/features/shell/lib/navigation";
import { useCurrentWorkspace } from "@/features/workspaces/session-context";
import { errorMessage, isApiError } from "@/lib/api/errors";
import { formatDateTime } from "@/lib/format";
import type { AudioTrack, Run } from "@/lib/api/types";

import { useJudgement } from "../api/judgement";
import { runAudioUrl, runQueryKey, useCancelRun, useRerunRun, useRun } from "../api/runs";
import { speakingTurn, type LiveTurn } from "../lib/events";
import { canOperateRuns } from "../lib/permissions";
import { formatEndReason, isActiveStatus } from "../lib/status";
import { CopyableValue } from "./copyable-value";
import { EvaluationPanel } from "./evaluation-panel";
import { RunLog } from "./run-log";
import { RunMetrics } from "./run-metrics";
import { RunStatusBadge } from "./run-status-badge";
import { TraceStatsCard } from "./trace-stats-card";
import { Transcript, type TurnFocus } from "./transcript";
import { useRunEvents } from "./use-run-events";

const TRACK_ORDER: AudioTrack[] = ["conversation", "user", "agent"];
const TRACK_LABELS: Record<AudioTrack, string> = {
  conversation: "Conversation",
  user: "Simulated user",
  agent: "Agent",
};

function DetailSkeleton() {
  return (
    <PageContainer className="gap-6">
      <span className="sr-only" role="status">
        Loading run
      </span>
      <Skeleton className="h-4 w-20" />
      <Skeleton className="h-9 w-72" />
      <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,1fr)_22rem]">
        <Skeleton className="h-96 rounded-xl" />
        <Skeleton className="h-72 rounded-xl" />
      </div>
    </PageContainer>
  );
}

export function RunDetailPage() {
  const { runId } = useParams<{ runId: string }>();
  // A fresh live state for every run.
  return <RunDetail key={runId} runId={runId} />;
}

function RunDetail({ runId }: { runId: string }) {
  const workspace = useCurrentWorkspace();
  const slug = workspace.slug;
  const runsHref = sectionHref(slug, SECTIONS.runs);
  const run = useRun(slug, runId);
  const queryClient = useQueryClient();
  const refresh = useCallback(
    () => queryClient.invalidateQueries({ queryKey: runQueryKey(slug, runId) }),
    [queryClient, slug, runId],
  );
  // The stream replays every event before `end`, so it also fills the log of finished runs.
  const { state: live, stream } = useRunEvents(slug, runId, run.data !== undefined, refresh);

  if (run.isPending) return <DetailSkeleton />;
  if (run.isError) {
    const notFound = isApiError(run.error) && (run.error.status === 404 || run.error.status === 422);
    return (
      <PageContainer>
        <EmptyState
          icon={PlayIcon}
          title={notFound ? "Run not found" : "Could not load the run"}
          description={notFound ? "It may belong to another workspace." : errorMessage(run.error)}
          action={
            notFound ? (
              <Button variant="outline" asChild>
                <Link href={runsHref}>Back to runs</Link>
              </Button>
            ) : (
              <Button variant="outline" onClick={() => run.refetch()}>
                <RotateCwIcon />
                Try again
              </Button>
            )
          }
        />
      </PageContainer>
    );
  }

  return <RunView run={run.data} live={live} streamClosed={stream === "closed"} slug={slug} canOperate={canOperateRuns(workspace.role)} />;
}

type RunViewProps = {
  run: Run;
  live: ReturnType<typeof useRunEvents>["state"];
  streamClosed: boolean;
  slug: string;
  canOperate: boolean;
};

function RunView({ run, live, streamClosed, slug, canOperate }: RunViewProps) {
  const router = useRouter();
  const cancel = useCancelRun(slug);
  const rerun = useRerunRun(slug);
  const runsHref = sectionHref(slug, SECTIONS.runs);

  // While the stored run is active, the stream drives the page; afterwards the stored run does.
  const active = isActiveStatus(run.status);
  const status = active && live.status ? live.status : run.status;
  const stillActive = isActiveStatus(status);
  const endReason = active ? (live.endReason ?? run.endReason) : run.endReason;
  const error = active ? (live.error ?? run.error) : run.error;
  const metrics = active ? (live.metrics ?? run.metrics) : (run.metrics ?? live.metrics);
  const turns: LiveTurn[] = useMemo(
    () => (active ? live.turns : run.transcript.map((turn, index) => ({ ...turn, index, final: true }))),
    [active, live.turns, run.transcript],
  );
  const judgement = useJudgement(slug, run.id, run.status === "completed");
  const scenarioHref = run.scenarioId ? `${sectionHref(slug, SECTIONS.scenarios)}/${run.scenarioId}` : null;

  // Evidence links bring a transcript turn into view and highlight it briefly.
  const [focus, setFocus] = useState<TurnFocus | null>(null);
  const focusTimer = useRef<ReturnType<typeof setTimeout>>(undefined);
  const showTurn = useCallback((index: number) => {
    clearTimeout(focusTimer.current);
    setFocus({ index, nonce: Date.now() });
    focusTimer.current = setTimeout(() => setFocus(null), 2500);
  }, []);
  useEffect(() => () => clearTimeout(focusTimer.current), []);

  const tracks = TRACK_ORDER.filter((track) => run.audioTracks.includes(track));

  function cancelRun() {
    cancel.mutate(run.id, {
      onSuccess: () => toast.success("Run cancelled"),
      onError: (err) => toast.error(errorMessage(err)),
    });
  }

  function rerunRun() {
    rerun.mutate(run.id, {
      onSuccess: (next) => {
        toast.success("Run queued again");
        router.push(`${runsHref}/${next.id}`);
      },
      onError: (err) => toast.error(errorMessage(err)),
    });
  }

  return (
    <PageContainer className="gap-6">
      <div className="flex flex-col gap-4">
        <Link
          href={`${runsHref}?batchId=${encodeURIComponent(run.batchId)}`}
          className="inline-flex w-fit items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"
        >
          <ArrowLeftIcon className="size-3.5" aria-hidden />
          Runs
        </Link>
        <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div className="flex min-w-0 flex-col gap-2">
            <div className="flex min-w-0 flex-wrap items-center gap-3">
              <h1 className="min-w-0 truncate text-2xl font-semibold tracking-tight">
                {scenarioHref ? (
                  <Link href={scenarioHref} className="hover:underline">
                    {run.scenarioName}
                  </Link>
                ) : (
                  run.scenarioName
                )}
              </h1>
              <RunStatusBadge status={status} />
              <ModalityBadge modality={run.modality} />
            </div>
            <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-sm text-muted-foreground">
              <span className="inline-flex items-center gap-1.5">
                <BotIcon aria-hidden className="size-3.5" />
                {run.agentName}
              </span>
              <span className="inline-flex items-center gap-1.5">
                <UserRoundIcon aria-hidden className="size-3.5" />
                {run.personaName}
              </span>
              <span className="tabular-nums">Attempt #{run.attempt}</span>
              {run.startedAt && <span>Started {formatDateTime(run.startedAt)}</span>}
              {!stillActive && endReason && <span>Ended: {formatEndReason(endReason)}</span>}
            </div>
          </div>
          {canOperate && (
            <div className="flex shrink-0 flex-wrap items-center gap-2">
              {stillActive && (
                <Button type="button" variant="outline" onClick={cancelRun} disabled={cancel.isPending}>
                  {cancel.isPending ? <Spinner /> : <SquareIcon />}
                  Cancel
                </Button>
              )}
              <Button type="button" variant="outline" onClick={rerunRun} disabled={rerun.isPending}>
                {rerun.isPending ? <Spinner /> : <RotateCcwIcon />}
                Rerun
              </Button>
            </div>
          )}
        </div>
        {error && (
          <div role="alert" className="flex items-start gap-2 rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2.5 text-sm">
            <CircleAlertIcon aria-hidden className="mt-0.5 size-4 shrink-0 text-destructive" />
            <span className="min-w-0 break-words">{error}</span>
          </div>
        )}
      </div>

      <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,1fr)_22rem]">
        <div className="flex min-w-0 flex-col gap-6">
          <Card className="min-w-0 gap-4">
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                Transcript
                {stillActive && <span className="text-xs font-normal text-muted-foreground">Live</span>}
              </CardTitle>
            </CardHeader>
            <CardContent>
              <Transcript turns={turns} live={stillActive} speaking={stillActive && speakingTurn(turns)} focus={focus} />
            </CardContent>
          </Card>
          <EvaluationPanel
            slug={slug}
            run={run}
            turns={turns}
            judgement={judgement}
            canOperate={canOperate}
            scenarioHref={scenarioHref}
            onShowTurn={showTurn}
          />
        </div>
        <aside className="flex min-w-0 flex-col gap-6 xl:sticky xl:top-20">
          <Card className="gap-4">
            <CardHeader>
              <CardTitle>Metrics</CardTitle>
            </CardHeader>
            <CardContent>
              <RunMetrics metrics={metrics} modality={run.modality} />
            </CardContent>
          </Card>
          {run.status === "completed" && <TraceStatsCard judgement={judgement.data} />}
          {run.modality === "voice" && (
            <Card className="gap-4">
              <CardHeader>
                <CardTitle className="flex items-center gap-2">
                  <AudioLinesIcon aria-hidden className="size-4 text-brand" />
                  Audio
                </CardTitle>
              </CardHeader>
              <CardContent className="flex flex-col gap-3">
                {tracks.length === 0 && (
                  <p className="text-sm text-muted-foreground">
                    {stillActive ? "Recordings are available when the run finishes." : "No recordings were stored."}
                  </p>
                )}
                {tracks.map((track) => (
                  <div key={track} className="flex flex-col gap-1">
                    <span className="text-xs text-muted-foreground">{TRACK_LABELS[track]}</span>
                    <audio controls preload="none" src={runAudioUrl(slug, run.id, track)} className="h-9 w-full" />
                  </div>
                ))}
              </CardContent>
            </Card>
          )}
          <Card className="gap-4">
            <CardHeader>
              <CardTitle>Identifiers</CardTitle>
            </CardHeader>
            <CardContent className="grid gap-3">
              <CopyableValue label="Thread id" value={run.threadId} />
              <CopyableValue label="Trace session id" value={run.traceSessionId} />
            </CardContent>
          </Card>
        </aside>
      </div>

      <RunLog logs={live.logs} loading={!live.ended && !streamClosed} />
    </PageContainer>
  );
}
