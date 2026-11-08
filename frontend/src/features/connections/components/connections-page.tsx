"use client";

import { PlugIcon, PlusIcon, RotateCwIcon } from "lucide-react";
import { useState } from "react";

import { EmptyState } from "@/components/empty-state";
import { PageContainer } from "@/components/page-container";
import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { canManageWorkspace, useCurrentWorkspace } from "@/features/workspaces/session-context";
import { errorMessage } from "@/lib/api/errors";
import type { Connection } from "@/lib/api/types";

import { useConnectionKinds, useConnections } from "../api/connections";
import { ConnectionDialog } from "./connection-dialog";
import { ConnectionList } from "./connection-list";
import { DeleteConnectionDialog } from "./delete-connection-dialog";
import { useConnectionTester } from "./use-connection-tester";

export function ConnectionsPage() {
  const workspace = useCurrentWorkspace();
  const canManage = canManageWorkspace(workspace.role);
  const canTest = workspace.role !== "viewer";
  const connections = useConnections(workspace.slug);
  const kinds = useConnectionKinds();
  const tester = useConnectionTester(workspace.slug);
  const [createOpen, setCreateOpen] = useState(false);
  const [editing, setEditing] = useState<Connection | null>(null);
  const [deleting, setDeleting] = useState<Connection | null>(null);

  const addButton = canManage ? (
    <Button onClick={() => setCreateOpen(true)}>
      <PlusIcon />
      Add connection
    </Button>
  ) : null;

  let body;
  if (connections.isError) {
    body = (
      <EmptyState
        icon={PlugIcon}
        title="Could not load connections"
        description={errorMessage(connections.error)}
        action={
          <Button variant="outline" onClick={() => connections.refetch()}>
            <RotateCwIcon />
            Try again
          </Button>
        }
      />
    );
  } else if (connections.data?.length === 0) {
    body = (
      <EmptyState
        icon={PlugIcon}
        title="No connections yet"
        description={
          canManage
            ? "Add the LLM and speech providers that simulate users and judge runs, and Langfuse to pull traces."
            : "Owners and admins of this workspace add the provider credentials TestForge uses."
        }
        action={addButton}
      />
    );
  } else {
    body = (
      <ConnectionList
        connections={connections.data}
        kinds={kinds.data}
        loading={connections.isPending}
        canTest={canTest}
        canManage={canManage}
        isTesting={tester.isTesting}
        onTest={(connection) => void tester.run(connection)}
        onEdit={setEditing}
        onDelete={setDeleting}
      />
    );
  }

  return (
    <PageContainer>
      <PageHeader
        title="Connections"
        description="Credentials for the models that simulate users and judge runs, and for Langfuse traces."
        actions={connections.data && connections.data.length > 0 ? addButton : null}
      />
      {body}
      {canManage && (
        <>
          <ConnectionDialog
            slug={workspace.slug}
            open={createOpen || editing !== null}
            connection={editing}
            onOpenChange={(open) => {
              if (!open) {
                setCreateOpen(false);
                setEditing(null);
              }
            }}
          />
          <DeleteConnectionDialog
            slug={workspace.slug}
            connection={deleting}
            onOpenChange={(open) => !open && setDeleting(null)}
          />
        </>
      )}
    </PageContainer>
  );
}
