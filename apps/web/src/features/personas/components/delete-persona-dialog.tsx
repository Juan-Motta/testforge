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
import type { Persona } from "@/lib/api/types";

import { useDeletePersona } from "../api/personas";

type DeletePersonaDialogProps = {
  slug: string;
  persona: Persona | null;
  onOpenChange: (open: boolean) => void;
};

export function DeletePersonaDialog({ slug, persona, onOpenChange }: DeletePersonaDialogProps) {
  const remove = useDeletePersona(slug);
  const [error, setError] = useState<string | null>(null);
  // Keep the last persona so the copy does not flash empty during the close animation.
  const [last, setLast] = useState(persona);
  if (persona && persona !== last) {
    setLast(persona);
    setError(null);
  }
  const shown = persona ?? last;

  function confirm(event: React.MouseEvent) {
    event.preventDefault();
    if (!persona) return;
    setError(null);
    remove.mutate(persona.id, {
      onSuccess: () => {
        toast.success(`Persona “${persona.name}” deleted`);
        onOpenChange(false);
      },
      // A 409 explains which scenarios still use it.
      onError: (cause) => setError(errorMessage(cause)),
    });
  }

  return (
    <AlertDialog open={persona !== null} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Delete “{shown?.name}”?</AlertDialogTitle>
          <AlertDialogDescription>
            {shown && shown.scenarioCount > 0
              ? `${shown.scenarioCount === 1 ? "1 scenario uses" : `${shown.scenarioCount} scenarios use`} this persona. Give them another persona before deleting it.`
              : "The persona is removed from this workspace. This cannot be undone."}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <FormError message={error ?? undefined} />
        <AlertDialogFooter>
          <AlertDialogCancel disabled={remove.isPending}>Cancel</AlertDialogCancel>
          <AlertDialogAction variant="destructive" onClick={confirm} disabled={remove.isPending}>
            {remove.isPending && <Spinner />}
            Delete persona
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
