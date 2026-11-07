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
import type { Scenario } from "@/lib/api/types";

import { useDeleteScenario } from "../api/scenarios";

type DeleteScenarioDialogProps = {
  slug: string;
  scenario: Pick<Scenario, "id" | "name">;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onDeleted: () => void;
};

export function DeleteScenarioDialog({ slug, scenario, open, onOpenChange, onDeleted }: DeleteScenarioDialogProps) {
  const remove = useDeleteScenario(slug);

  function confirm(event: React.MouseEvent) {
    event.preventDefault();
    remove.mutate(scenario.id, {
      onSuccess: () => {
        toast.success(`Scenario “${scenario.name}” deleted`);
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
          <AlertDialogTitle>Delete “{scenario.name}”?</AlertDialogTitle>
          <AlertDialogDescription>
            Its goal, facts and criteria are erased. The agent and persona stay. This cannot be undone.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={remove.isPending}>Cancel</AlertDialogCancel>
          <AlertDialogAction variant="destructive" onClick={confirm} disabled={remove.isPending}>
            {remove.isPending && <Spinner />}
            Delete scenario
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
