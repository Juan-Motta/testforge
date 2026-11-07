"use client";

import { BotIcon, PlusIcon, RotateCwIcon } from "lucide-react";
import Link from "next/link";

import { EmptyState } from "@/components/empty-state";
import { PageContainer } from "@/components/page-container";
import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { useConnections } from "@/features/connections/api/connections";
import { SECTIONS } from "@/features/shell/lib/navigation";
import { canManageWorkspace, useCurrentWorkspace } from "@/features/workspaces/session-context";
import { errorMessage } from "@/lib/api/errors";

import { useAgentCodecs, useAgents } from "../api/agents";
import { AgentList } from "./agent-list";

export function AgentsPage() {
  const workspace = useCurrentWorkspace();
  const canManage = canManageWorkspace(workspace.role);
  const agents = useAgents(workspace.slug);
  const codecs = useAgentCodecs();
  const connections = useConnections(workspace.slug);

  const newButton = canManage ? (
    <Button asChild>
      <Link href={`/w/${workspace.slug}/agents/new`}>
        <PlusIcon />
        New agent
      </Link>
    </Button>
  ) : null;

  let body;
  if (agents.isError) {
    body = (
      <EmptyState
        icon={BotIcon}
        title="Could not load agents"
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
        icon={BotIcon}
        title="No agents yet"
        description={
          canManage
            ? "Point TestForge at your voice or text agent: start from a preset or configure the WebSocket or HTTP endpoint yourself."
            : "Owners and admins of this workspace add the agents TestForge tests."
        }
        action={newButton}
      />
    );
  } else {
    body = (
      <AgentList
        slug={workspace.slug}
        agents={agents.data}
        codecs={codecs.data}
        connections={connections.data}
        loading={agents.isPending}
      />
    );
  }

  return (
    <PageContainer>
      <PageHeader
        title="Agents"
        description={SECTIONS.agents.description}
        actions={agents.data && agents.data.length > 0 ? newButton : null}
      />
      {body}
    </PageContainer>
  );
}
