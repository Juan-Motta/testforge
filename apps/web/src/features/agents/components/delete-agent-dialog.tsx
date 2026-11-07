"use client";

import { useState } from "react";
import { toast } from "sonner";

import { FormError } from "@/components/form-error";

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Spinner } from "@/components/ui/spinner";
import { errorMessage } from "@/lib/api/errors";
import type { Agent } from "@/lib/api/types";

import { useDeleteAgent } from "../api/agents";

type DeleteAgentDialogProps = {
  slug: string;
  agent: Pick<Agent, "id" | "name">;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onDeleted: () => void;
};

export function DeleteAgentDialog({ slug, agent, open, onOpenChange, onDeleted }: DeleteAgentDialogProps) {
  const remove = useDeleteAgent(slug);
  // Shown inside the dialog: a 409 explains how many scenarios still use the agent.
  const [error, setError] = useState<string | null>(null);

  function changeOpen(next: boolean) {
    if (!next) setError(null);
    onOpenChange(next);
  }

  function confirm(event: React.MouseEvent) {
    event.preventDefault();
    setError(null);
    remove.mutate(agent.id, {
      onSuccess: () => {
        toast.success(`Agent “${agent.name}” deleted`);
        changeOpen(false);
        onDeleted();
      },
      onError: (cause) => setError(errorMessage(cause)),
    });
  }

  return (
    <AlertDialog open={open} onOpenChange={changeOpen}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete “{agent.name}”?</AlertDialogTitle>
          <AlertDialogDescription>
            Its configuration and stored secrets are erased. Delete the scenarios that test this agent first.
            This cannot be undone.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <FormError message={error ?? undefined} />
        <AlertDialogFooter>
          <AlertDialogCancel disabled={remove.isPending}>Cancel</AlertDialogCancel>
          <AlertDialogAction variant="destructive" onClick={confirm} disabled={remove.isPending}>
            {remove.isPending && <Spinner />}
            Delete agent
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
