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
import type { SandboxAgent } from "@/lib/api/types";

import { useDeleteSandboxAgent } from "../api/sandbox";

type DeleteSandboxDialogProps = {
  slug: string;
  agent: Pick<SandboxAgent, "id" | "name">;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onDeleted: () => void;
};

export function DeleteSandboxDialog({ slug, agent, open, onOpenChange, onDeleted }: DeleteSandboxDialogProps) {
  const remove = useDeleteSandboxAgent(slug);
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
        toast.success(`Sandbox agent “${agent.name}” deleted`);
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
            Its conversations and memories are erased. Agents linked to it stay but fail until you edit them. This
            cannot be undone.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <FormError message={error ?? undefined} />
        <AlertDialogFooter>
          <AlertDialogCancel disabled={remove.isPending}>Cancel</AlertDialogCancel>
          <AlertDialogAction variant="destructive" onClick={confirm} disabled={remove.isPending}>
            {remove.isPending && <Spinner />}
            Delete sandbox agent
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
