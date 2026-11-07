"use client";

import { useState } from "react";
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
import type { ScenarioGroup } from "@/lib/api/types";

import { useDeleteScenarioGroup } from "../api/scenarios";

type DeleteGroupDialogProps = {
  slug: string;
  group: ScenarioGroup | null;
  onOpenChange: (open: boolean) => void;
  onDeleted: (group: ScenarioGroup) => void;
};

export function DeleteGroupDialog({ slug, group, onOpenChange, onDeleted }: DeleteGroupDialogProps) {
  const remove = useDeleteScenarioGroup(slug);
  const [last, setLast] = useState(group);
  if (group && group !== last) setLast(group);
  const shown = group ?? last;

  function confirm(event: React.MouseEvent) {
    event.preventDefault();
    if (!group) return;
    remove.mutate(group.id, {
      onSuccess: () => {
        toast.success(`Group “${group.name}” deleted`);
        onOpenChange(false);
        onDeleted(group);
      },
      onError: (error) => toast.error(errorMessage(error)),
    });
  }

  const count = shown?.scenarioCount ?? 0;
  return (
    <AlertDialog open={group !== null} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete the group “{shown?.name}”?</AlertDialogTitle>
          <AlertDialogDescription>
            {count === 0
              ? "The group is empty. This cannot be undone."
              : `Its ${count === 1 ? "scenario is" : `${count} scenarios are`} kept and become ungrouped. This cannot be undone.`}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={remove.isPending}>Cancel</AlertDialogCancel>
          <AlertDialogAction variant="destructive" onClick={confirm} disabled={remove.isPending}>
            {remove.isPending && <Spinner />}
            Delete group
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
