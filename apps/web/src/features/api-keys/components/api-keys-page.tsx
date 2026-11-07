"use client";

import { KeyRoundIcon, PlusIcon, RotateCwIcon } from "lucide-react";
import { useState } from "react";

import { EmptyState } from "@/components/empty-state";
import { Button } from "@/components/ui/button";
import { canManageWorkspace, useCurrentWorkspace } from "@/features/workspaces/session-context";
import { errorMessage } from "@/lib/api/errors";
import type { ApiKey } from "@/lib/api/types";

import { useApiKeys } from "../api/api-keys";
import { ApiKeysTable } from "./api-keys-table";
import { CreateApiKeyDialog } from "./create-api-key-dialog";
import { RevokeApiKeyDialog } from "./revoke-api-key-dialog";

export function ApiKeysPage() {
  const workspace = useCurrentWorkspace();
  const canManage = canManageWorkspace(workspace.role);
  const keys = useApiKeys(workspace.slug);
  const [createOpen, setCreateOpen] = useState(false);
  const [revoking, setRevoking] = useState<ApiKey | null>(null);

  const createButton = canManage ? (
    <Button onClick={() => setCreateOpen(true)}>
      <PlusIcon />
      Create API key
    </Button>
  ) : null;

  let body;
  if (keys.isError) {
    body = (
      <EmptyState
        icon={KeyRoundIcon}
        title="Could not load API keys"
        description={errorMessage(keys.error)}
        action={
          <Button variant="outline" onClick={() => keys.refetch()}>
            <RotateCwIcon />
            Try again
          </Button>
        }
      />
    );
  } else if (keys.data?.length === 0) {
    body = (
      <EmptyState
        icon={KeyRoundIcon}
        title="No API keys yet"
        description={
          canManage
            ? "Create a key to call the TestForge API from scripts, CI pipelines and the CLI."
            : "Owners and admins of this workspace can create keys for scripts, CI pipelines and the CLI."
        }
        action={createButton}
      />
    );
  } else {
    body = (
      <ApiKeysTable
        keys={keys.data}
        loading={keys.isPending}
        canManage={canManage}
        onRevoke={setRevoking}
      />
    );
  }

  return (
    <section className="flex flex-col gap-6" aria-labelledby="api-keys-heading">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
        <div className="flex flex-col gap-1">
          <h2 id="api-keys-heading" className="text-lg font-semibold tracking-tight">
            API keys
          </h2>
          <p className="text-sm text-muted-foreground">
            Keys authenticate requests with <code className="font-mono text-xs">Authorization: Bearer tf_…</code> and
            only reach this workspace.
          </p>
        </div>
        {keys.data && keys.data.length > 0 && createButton}
      </div>
      {body}
      {canManage && (
        <>
          <CreateApiKeyDialog slug={workspace.slug} open={createOpen} onOpenChange={setCreateOpen} />
          <RevokeApiKeyDialog
            slug={workspace.slug}
            apiKey={revoking}
            onOpenChange={(open) => !open && setRevoking(null)}
          />
        </>
      )}
    </section>
  );
}
