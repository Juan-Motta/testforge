"use client";

import { ClipboardListIcon, PlayIcon, RotateCwIcon, SearchXIcon } from "lucide-react";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useCallback, useMemo, useState } from "react";
import { toast } from "sonner";

import { EmptyState } from "@/components/empty-state";
import { PageContainer } from "@/components/page-container";
import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import { useAgents } from "@/features/agents/api/agents";
import { SECTIONS, sectionHref } from "@/features/shell/lib/navigation";
import { useCurrentWorkspace } from "@/features/workspaces/session-context";
import { errorMessage } from "@/lib/api/errors";
import type { RunBatch } from "@/lib/api/types";

import { useCancelBatch, useRunBatches, useRuns } from "../api/runs";
import { filtersFromSearchParams, filtersToQueryString, NO_RUN_FILTERS, type RunFilters } from "../lib/filters";
import { canOperateRuns } from "../lib/permissions";
import { BatchList, batchTitle } from "./batch-list";
import { RunsTable } from "./runs-table";
import { RunsToolbar } from "./runs-toolbar";

export function RunsPage() {
  const workspace = useCurrentWorkspace();
  const slug = workspace.slug;
  const canOperate = canOperateRuns(workspace.role);
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const queryString = searchParams.toString();
  const filters = useMemo(() => filtersFromSearchParams(new URLSearchParams(queryString)), [queryString]);

  const agents = useAgents(slug);
  const batches = useRunBatches(slug);
  const runs = useRuns(slug, filters);
  const cancelBatch = useCancelBatch(slug);
  const [cancellingId, setCancellingId] = useState<string | null>(null);

  const setFilters = useCallback(
    (patch: Partial<RunFilters>) => {
      const current = filtersFromSearchParams(new URLSearchParams(window.location.search));
      const next = filtersToQueryString({ ...current, ...patch });
      router.replace(next ? `${pathname}?${next}` : pathname, { scroll: false });
    },
    [router, pathname],
  );

  function cancel(batch: RunBatch) {
    setCancellingId(batch.id);
    cancelBatch.mutate(batch.id, {
      onSuccess: () => toast.success(`Cancelled unfinished runs of “${batchTitle(batch)}”`),
      onError: (error) => toast.error(errorMessage(error)),
      onSettled: () => setCancellingId(null),
    });
  }

  const items = runs.data?.pages.flatMap((page) => page.items);
  const filtered = filters.batchId !== null || filters.status !== null || filters.agentId !== null;
  const noRunsAtAll = items?.length === 0 && !filtered && batches.data?.length === 0;

  let body;
  if (runs.isError) {
    body = (
      <EmptyState
        icon={PlayIcon}
        title="Could not load runs"
        description={errorMessage(runs.error)}
        action={
          <Button variant="outline" onClick={() => runs.refetch()}>
            <RotateCwIcon />
            Try again
          </Button>
        }
      />
    );
  } else if (items?.length === 0 && !filtered) {
    body = (
      <EmptyState
        icon={PlayIcon}
        title="No runs yet"
        description={
          canOperate
            ? "Open Scenarios, select the ones to test (or open one) and choose Run. Each run plays a scenario against its agent with a simulated user."
            : "Members of this workspace launch runs from Scenarios. They will show up here with their transcripts and metrics."
        }
        action={
          <Button variant="outline" asChild>
            <Link href={sectionHref(slug, SECTIONS.scenarios)}>
              <ClipboardListIcon />
              Open scenarios
            </Link>
          </Button>
        }
      />
    );
  } else if (items?.length === 0) {
    body = (
      <EmptyState
        icon={SearchXIcon}
        title="No runs match"
        description="Try another batch, status or agent."
        className="min-h-60"
        action={
          <Button variant="outline" onClick={() => setFilters(NO_RUN_FILTERS)}>
            Show all runs
          </Button>
        }
      />
    );
  } else {
    body = (
      <div className="flex flex-col gap-3">
        <RunsTable slug={slug} runs={items} loading={runs.isPending} />
        {runs.hasNextPage && (
          <Button
            variant="outline"
            className="self-center"
            onClick={() => runs.fetchNextPage()}
            disabled={runs.isFetchingNextPage}
          >
            {runs.isFetchingNextPage && <Spinner />}
            Load more
          </Button>
        )}
      </div>
    );
  }

  return (
    <PageContainer>
      <PageHeader title="Runs" description={SECTIONS.runs.description} />
      {noRunsAtAll ? (
        body
      ) : (
        <div className="grid items-start gap-6 lg:grid-cols-[15rem_minmax(0,1fr)]">
          <BatchList
            batches={batches.data}
            selected={filters.batchId}
            canOperate={canOperate}
            cancellingId={cancellingId}
            onSelect={(batchId) => setFilters({ batchId })}
            onCancel={cancel}
          />
          <div className="flex min-w-0 flex-col gap-4">
            <RunsToolbar filters={filters} batches={batches.data} agents={agents.data} onChange={setFilters} />
            {body}
          </div>
        </div>
      )}
    </PageContainer>
  );
}
