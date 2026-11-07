"use client";

import { PlusIcon, RotateCwIcon, UsersRoundIcon } from "lucide-react";
import { useState } from "react";

import { EmptyState } from "@/components/empty-state";
import { PageContainer } from "@/components/page-container";
import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { useConnections } from "@/features/connections/api/connections";
import { canAuthorScenarios } from "@/features/scenarios/lib/permissions";
import { SECTIONS } from "@/features/shell/lib/navigation";
import { useCurrentWorkspace } from "@/features/workspaces/session-context";
import { errorMessage } from "@/lib/api/errors";
import type { Persona } from "@/lib/api/types";

import { usePersonas } from "../api/personas";
import { DeletePersonaDialog } from "./delete-persona-dialog";
import { PersonaDialog } from "./persona-dialog";
import { PersonaList } from "./persona-list";

export function PersonasPage() {
  const workspace = useCurrentWorkspace();
  const canAuthor = canAuthorScenarios(workspace.role);
  const personas = usePersonas(workspace.slug);
  const connections = useConnections(workspace.slug);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editing, setEditing] = useState<Persona | null>(null);
  const [deleting, setDeleting] = useState<Persona | null>(null);

  function openNew() {
    setEditing(null);
    setDialogOpen(true);
  }

  const newButton = canAuthor ? (
    <Button onClick={openNew}>
      <PlusIcon />
      New persona
    </Button>
  ) : null;

  let body;
  if (personas.isError) {
    body = (
      <EmptyState
        icon={UsersRoundIcon}
        title="Could not load personas"
        description={errorMessage(personas.error)}
        action={
          <Button variant="outline" onClick={() => personas.refetch()}>
            <RotateCwIcon />
            Try again
          </Button>
        }
      />
    );
  } else if (personas.data?.length === 0) {
    body = (
      <EmptyState
        icon={UsersRoundIcon}
        title="No personas yet"
        description={
          canAuthor
            ? "Create the simulated users your scenarios reuse: a temperament, a language and, for voice agents, a voice."
            : "Members of this workspace create the simulated users that talk to your agents."
        }
        action={newButton}
      />
    );
  } else {
    body = (
      <PersonaList
        personas={personas.data}
        connections={connections.data}
        loading={personas.isPending}
        canAuthor={canAuthor}
        onEdit={(persona) => {
          setEditing(persona);
          setDialogOpen(true);
        }}
        onDelete={setDeleting}
      />
    );
  }

  return (
    <PageContainer>
      <PageHeader
        title="Personas"
        description={SECTIONS.personas.description}
        actions={personas.data && personas.data.length > 0 ? newButton : null}
      />
      {body}
      {canAuthor && (
        <>
          <PersonaDialog slug={workspace.slug} open={dialogOpen} onOpenChange={setDialogOpen} persona={editing} />
          <DeletePersonaDialog slug={workspace.slug} persona={deleting} onOpenChange={(open) => !open && setDeleting(null)} />
        </>
      )}
    </PageContainer>
  );
}
