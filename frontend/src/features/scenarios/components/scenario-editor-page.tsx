"use client";

import { ClipboardListIcon, RotateCwIcon } from "lucide-react";
import Link from "next/link";
import { useParams, useRouter, useSearchParams } from "next/navigation";

import { EmptyState } from "@/components/empty-state";
import { PageContainer } from "@/components/page-container";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { useAgents } from "@/features/agents/api/agents";
import { SECTIONS, sectionHref } from "@/features/shell/lib/navigation";
import { useCurrentWorkspace } from "@/features/workspaces/session-context";
import { errorMessage, isApiError } from "@/lib/api/errors";

import { useScenario } from "../api/scenarios";
import { canAuthorScenarios } from "../lib/permissions";
import { ScenarioEditor } from "./scenario-editor";

function EditorSkeleton() {
  return (
    <PageContainer className="gap-6">
      <span className="sr-only" role="status">
        Loading scenario
      </span>
      <Skeleton className="h-4 w-20" />
      <Skeleton className="h-9 w-72" />
      <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,1fr)_22rem]">
        <div className="flex flex-col gap-6">
          <Skeleton className="h-48 rounded-xl" />
          <Skeleton className="h-72 rounded-xl" />
          <Skeleton className="h-64 rounded-xl" />
        </div>
        <Skeleton className="h-96 rounded-xl" />
      </div>
    </PageContainer>
  );
}

function BackToScenarios({ slug }: { slug: string }) {
  return (
    <Button variant="outline" asChild>
      <Link href={sectionHref(slug, SECTIONS.scenarios)}>Back to scenarios</Link>
    </Button>
  );
}

function LoadError({ title, error, onRetry }: { title: string; error: unknown; onRetry: () => void }) {
  return (
    <PageContainer>
      <EmptyState
        icon={ClipboardListIcon}
        title={title}
        description={errorMessage(error)}
        action={
          <Button variant="outline" onClick={onRetry}>
            <RotateCwIcon />
            Try again
          </Button>
        }
      />
    </PageContainer>
  );
}

export function NewScenarioPage() {
  const workspace = useCurrentWorkspace();
  const searchParams = useSearchParams();
  // Pages stay mounted while hidden; a new visit (push/replace) starts a fresh form,
  // while browser back/forward restores the draft.
  const { bfcacheId } = useRouter();
  const agents = useAgents(workspace.slug);

  if (!canAuthorScenarios(workspace.role)) {
    return (
      <PageContainer>
        <EmptyState
          icon={ClipboardListIcon}
          title="You can't add scenarios"
          description="Viewers can read scenarios; owners, admins and members write them."
          action={<BackToScenarios slug={workspace.slug} />}
        />
      </PageContainer>
    );
  }
  if (agents.isPending) return <EditorSkeleton />;
  if (agents.isError) return <LoadError title="Could not load agents" error={agents.error} onRetry={() => agents.refetch()} />;
  if (agents.data.length === 0) {
    return (
      <PageContainer>
        <EmptyState
          icon={ClipboardListIcon}
          title="Add an agent first"
          description="Every scenario tests one agent. Connect it in Agents, then write its scenarios."
          action={
            <Button variant="outline" asChild>
              <Link href={sectionHref(workspace.slug, SECTIONS.agents)}>Open agents</Link>
            </Button>
          }
        />
      </PageContainer>
    );
  }
  const prefill = {
    agentId: searchParams.get("agentId") ?? undefined,
    groupId: searchParams.get("groupId") ?? undefined,
  };
  return <ScenarioEditor key={bfcacheId} agents={agents.data} prefill={prefill} />;
}

export function EditScenarioPage() {
  const workspace = useCurrentWorkspace();
  const { scenarioId } = useParams<{ scenarioId: string }>();
  const { bfcacheId } = useRouter();
  const scenario = useScenario(workspace.slug, scenarioId);
  const agents = useAgents(workspace.slug);

  if (scenario.isPending || agents.isPending) return <EditorSkeleton />;
  if (agents.isError) return <LoadError title="Could not load the scenario" error={agents.error} onRetry={() => agents.refetch()} />;
  if (scenario.isError) {
    const notFound = isApiError(scenario.error) && (scenario.error.status === 404 || scenario.error.status === 422);
    if (!notFound) {
      return <LoadError title="Could not load the scenario" error={scenario.error} onRetry={() => scenario.refetch()} />;
    }
    return (
      <PageContainer>
        <EmptyState
          icon={ClipboardListIcon}
          title="Scenario not found"
          description="It may have been deleted."
          action={<BackToScenarios slug={workspace.slug} />}
        />
      </PageContainer>
    );
  }
  // Remount when another scenario is opened, or the page is visited again, so the form
  // starts from the stored values.
  return <ScenarioEditor key={`${scenario.data.id}-${bfcacheId}`} agents={agents.data} scenario={scenario.data} />;
}
