"use client";

import { LayersIcon, PlusIcon } from "lucide-react";
import { useState } from "react";

import { EmptyState } from "@/components/empty-state";
import { Logo } from "@/components/logo";
import { Button } from "@/components/ui/button";

import { CreateWorkspaceDialog } from "./create-workspace-dialog";

/** Shown at `/` when the user does not belong to any workspace. */
export function NoWorkspace() {
  const [open, setOpen] = useState(false);
  return (
    <div className="flex min-h-svh flex-col items-center justify-center gap-10 px-4">
      <Logo />
      <EmptyState
        className="max-w-lg"
        icon={LayersIcon}
        title="You are not in any workspace yet"
        description="Create a workspace to start connecting agents and running tests, or ask a teammate to invite you."
        action={
          <Button onClick={() => setOpen(true)}>
            <PlusIcon />
            Create workspace
          </Button>
        }
      />
      <CreateWorkspaceDialog open={open} onOpenChange={setOpen} />
    </div>
  );
}
