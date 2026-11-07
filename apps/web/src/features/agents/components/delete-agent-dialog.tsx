"use client";

import { toast } from "sonner";

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

  function confirm(event: React.MouseEvent) {
    event.preventDefault();
    remove.mutate(agent.id, {
      onSuccess: () => {
        toast.success(`Agent “${agent.name}” deleted`);
        onOpenChange(false);
        onDeleted();
      },
      onError: (error) => toast.error(errorMessage(error)),
    });
  }

  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete “{agent.name}”?</AlertDialogTitle>
          <AlertDialogDescription>
            Its configuration and stored secrets are erased. Scenarios and runs that target this agent will
            need another one. This cannot be undone.
          </AlertDialogDescription>
        </AlertDialogHeader>
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
