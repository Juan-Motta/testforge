"use client";

import { FlaskConicalIcon, RotateCwIcon } from "lucide-react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";

import { EmptyState } from "@/components/empty-state";
import { PageContainer } from "@/components/page-container";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { sectionHref, SECTIONS } from "@/features/shell/lib/navigation";
import { useCurrentWorkspace } from "@/features/workspaces/session-context";
import { errorMessage, isApiError } from "@/lib/api/errors";

import { useSandboxAgent } from "../api/sandbox";
import { canEditSandbox } from "../lib/permissions";
import { SandboxEditor } from "./sandbox-editor";

function EditorSkeleton() {
  return (
    <PageContainer className="gap-6">
      <span className="sr-only" role="status">
        Loading sandbox agent
      </span>
      <Skeleton className="h-4 w-16" />
      <Skeleton className="h-9 w-64" />
      <div className="grid items-start gap-6 lg:grid-cols-[minmax(0,1fr)_24rem]">
        <div className="flex flex-col gap-6">
          <Skeleton className="h-64 rounded-xl" />
          <Skeleton className="h-48 rounded-xl" />
          <Skeleton className="h-40 rounded-xl" />
        </div>
        <Skeleton className="h-96 rounded-xl" />
      </div>
    </PageContainer>
  );
}

function BackToSandbox({ slug }: { slug: string }) {
  return (
    <Button variant="outline" asChild>
      <Link href={sectionHref(slug, SECTIONS.sandbox)}>Back to sandbox</Link>
    </Button>
  );
}

export function NewSandboxAgentPage() {
  const workspace = useCurrentWorkspace();
  // Pages stay mounted while hidden; a new visit (push/replace) starts a fresh form,
  // while browser back/forward restores the draft.
  const { bfcacheId } = useRouter();
  if (!canEditSandbox(workspace.role)) {
    return (
      <PageContainer>
        <EmptyState
          icon={FlaskConicalIcon}
          title="You can't add sandbox agents"
          description="Viewers can't create sandbox agents."
          action={<BackToSandbox slug={workspace.slug} />}
        />
      </PageContainer>
    );
  }
  return <SandboxEditor key={bfcacheId} />;
}

export function EditSandboxAgentPage() {
  const workspace = useCurrentWorkspace();
  const { sandboxId } = useParams<{ sandboxId: string }>();
  const { bfcacheId } = useRouter();
  const agent = useSandboxAgent(workspace.slug, sandboxId);

  if (agent.isPending) return <EditorSkeleton />;
  if (agent.isError) {
    const notFound = isApiError(agent.error) && (agent.error.status === 404 || agent.error.status === 422);
    return (
      <PageContainer>
        <EmptyState
          icon={FlaskConicalIcon}
          title={notFound ? "Sandbox agent not found" : "Could not load the sandbox agent"}
          description={notFound ? "It may have been deleted." : errorMessage(agent.error)}
          action={
            notFound ? (
              <BackToSandbox slug={workspace.slug} />
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
  // Remount when another agent is opened, or the page is visited again, so the form starts from the stored values.
  return <SandboxEditor key={`${agent.data.id}-${bfcacheId}`} agent={agent.data} />;
}
