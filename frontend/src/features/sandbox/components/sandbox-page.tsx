"use client";

import { FlaskConicalIcon, PlusIcon, RotateCwIcon } from "lucide-react";
import Link from "next/link";

import { EmptyState } from "@/components/empty-state";
import { PageContainer } from "@/components/page-container";
import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { useConnections } from "@/features/connections/api/connections";
import { sectionHref, SECTIONS } from "@/features/shell/lib/navigation";
import { useCurrentWorkspace } from "@/features/workspaces/session-context";
import { errorMessage } from "@/lib/api/errors";

import { useSandboxAgents } from "../api/sandbox";
import { canEditSandbox } from "../lib/permissions";
import { SandboxList } from "./sandbox-list";

export function SandboxPage() {
  const workspace = useCurrentWorkspace();
  const canEdit = canEditSandbox(workspace.role);
  const agents = useSandboxAgents(workspace.slug);
  const connections = useConnections(workspace.slug);
  const sandboxHref = sectionHref(workspace.slug, SECTIONS.sandbox);

  const newButton = canEdit ? (
    <Button asChild>
      <Link href={`${sandboxHref}/new`}>
        <PlusIcon />
        New sandbox agent
      </Link>
    </Button>
  ) : null;

  let body;
  if (agents.isError) {
    body = (
      <EmptyState
        icon={FlaskConicalIcon}
        title="Could not load sandbox agents"
        description={errorMessage(agents.error)}
        action={
          <Button variant="outline" onClick={() => agents.refetch()}>
            <RotateCwIcon />
            Try again
          </Button>
        }
      />
    );
  } else if (agents.data?.length === 0) {
    body = (
      <EmptyState
        icon={FlaskConicalIcon}
        title="No sandbox agents yet"
        description={
          canEdit
            ? "Describe an agent, give it mock tools and memory, chat with it here and then test it with scenarios."
            : "Owners, admins and members of this workspace build sandbox agents."
        }
        action={newButton}
      />
    );
  } else {
    body = (
      <SandboxList
        hrefFor={(id) => `${sandboxHref}/${id}`}
        agents={agents.data}
        connections={connections.data}
        loading={agents.isPending}
      />
    );
  }

  return (
    <PageContainer>
      <PageHeader
        title="Sandbox"
        description={SECTIONS.sandbox.description}
        actions={agents.data && agents.data.length > 0 ? newButton : null}
      />
      {body}
    </PageContainer>
  );
}
