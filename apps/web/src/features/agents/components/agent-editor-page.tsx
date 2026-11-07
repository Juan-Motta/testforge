"use client";

import { BotIcon, RotateCwIcon } from "lucide-react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";

import { EmptyState } from "@/components/empty-state";
import { PageContainer } from "@/components/page-container";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { canManageWorkspace, useCurrentWorkspace } from "@/features/workspaces/session-context";
import { errorMessage, isApiError } from "@/lib/api/errors";

import { useAgent, useAgentCodecs } from "../api/agents";
import { AgentEditor } from "./agent-editor";

function EditorSkeleton() {
  return (
    <PageContainer className="gap-6">
      <span className="sr-only" role="status">
        Loading agent
      </span>
      <Skeleton className="h-4 w-16" />
      <Skeleton className="h-9 w-64" />
      <div className="grid items-start gap-6 lg:grid-cols-[minmax(0,1fr)_22rem]">
        <div className="flex flex-col gap-6">
          <Skeleton className="h-40 rounded-xl" />
          <Skeleton className="h-80 rounded-xl" />
          <Skeleton className="h-40 rounded-xl" />
        </div>
        <Skeleton className="h-72 rounded-xl" />
      </div>
    </PageContainer>
  );
}

function BackToAgents({ slug }: { slug: string }) {
  return (
    <Button variant="outline" asChild>
      <Link href={`/w/${slug}/agents`}>Back to agents</Link>
    </Button>
  );
}

export function NewAgentPage() {
  const workspace = useCurrentWorkspace();
  // Pages stay mounted while hidden; a new visit (push/replace) starts a fresh form,
  // while browser back/forward restores the draft.
  const { bfcacheId } = useRouter();
  if (!canManageWorkspace(workspace.role)) {
    return (
      <PageContainer>
        <EmptyState
          icon={BotIcon}
          title="You can't add agents"
          description="Only owners and admins of this workspace add agents."
          action={<BackToAgents slug={workspace.slug} />}
        />
      </PageContainer>
    );
  }
  return <AgentEditor key={bfcacheId} />;
}

export function EditAgentPage() {
  const workspace = useCurrentWorkspace();
  const { agentId } = useParams<{ agentId: string }>();
  const { bfcacheId } = useRouter();
  const agent = useAgent(workspace.slug, agentId);
  const codecs = useAgentCodecs();

  if (agent.isPending || codecs.isPending) return <EditorSkeleton />;
  if (codecs.isError) {
    return (
      <PageContainer>
        <EmptyState
          icon={BotIcon}
          title="Could not load the agent"
          description={errorMessage(codecs.error)}
          action={
            <Button variant="outline" onClick={() => codecs.refetch()}>
              <RotateCwIcon />
              Try again
            </Button>
          }
        />
      </PageContainer>
    );
  }
  if (agent.isError) {
    const notFound = isApiError(agent.error) && (agent.error.status === 404 || agent.error.status === 422);
    return (
      <PageContainer>
        <EmptyState
          icon={BotIcon}
          title={notFound ? "Agent not found" : "Could not load the agent"}
          description={notFound ? "It may have been deleted." : errorMessage(agent.error)}
          action={
            notFound ? (
              <BackToAgents slug={workspace.slug} />
            ) : (
              <Button variant="outline" onClick={() => agent.refetch()}>
                <RotateCwIcon />
                Try again
              </Button>
            )
          }
        />
      </PageContainer>
    );
  }
  // Remount when another agent is opened, or the page is visited again, so the form
  // starts from the stored values.
  return <AgentEditor key={`${agent.data.id}-${bfcacheId}`} agent={agent.data} />;
}
