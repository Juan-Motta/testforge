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
import type { Connection } from "@/lib/api/types";

import { useDeleteConnection } from "../api/connections";

type DeleteConnectionDialogProps = {
  slug: string;
  connection: Connection | null;
  onOpenChange: (open: boolean) => void;
};

export function DeleteConnectionDialog({ slug, connection, onOpenChange }: DeleteConnectionDialogProps) {
  const remove = useDeleteConnection(slug);
  // Keep the last connection so the copy does not flash empty during the close animation.
  const [last, setLast] = useState(connection);
  if (connection && connection !== last) setLast(connection);
  const shown = connection ?? last;

  function confirm(event: React.MouseEvent) {
    event.preventDefault();
    if (!connection) return;
    remove.mutate(connection.id, {
      onSuccess: () => {
        toast.success(`Connection “${connection.name}” deleted`);
        onOpenChange(false);
      },
      onError: (error) => toast.error(errorMessage(error)),
    });
  }

  return (
    <AlertDialog open={connection !== null} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete “{shown?.name}”?</AlertDialogTitle>
          <AlertDialogDescription>
            Its stored secret is erased, and any runs or scenarios that use this connection will
            stop working until you pick another one. This cannot be undone.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={remove.isPending}>Cancel</AlertDialogCancel>
          <AlertDialogAction variant="destructive" onClick={confirm} disabled={remove.isPending}>
            {remove.isPending && <Spinner />}
            Delete connection
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
