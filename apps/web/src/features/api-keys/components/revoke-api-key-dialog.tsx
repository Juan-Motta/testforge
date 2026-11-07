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
import type { ApiKey } from "@/lib/api/types";

import { useRevokeApiKey } from "../api/api-keys";

type RevokeApiKeyDialogProps = {
  slug: string;
  apiKey: ApiKey | null;
  onOpenChange: (open: boolean) => void;
};

export function RevokeApiKeyDialog({ slug, apiKey, onOpenChange }: RevokeApiKeyDialogProps) {
  const revoke = useRevokeApiKey(slug);
  // Keep the last key so the copy does not flash empty during the close animation.
  const [lastKey, setLastKey] = useState(apiKey);
  if (apiKey && apiKey !== lastKey) setLastKey(apiKey);
  const shown = apiKey ?? lastKey;

  function confirm(event: React.MouseEvent) {
    event.preventDefault();
    if (!apiKey) return;
    revoke.mutate(apiKey.id, {
      onSuccess: () => {
        toast.success(`API key “${apiKey.name}” revoked`);
        onOpenChange(false);
      },
      onError: (error) => toast.error(errorMessage(error)),
    });
  }

  return (
    <AlertDialog open={apiKey !== null} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Revoke “{shown?.name}”?</AlertDialogTitle>
          <AlertDialogDescription>
            Anything using the key <code className="font-mono text-xs">{shown?.prefix}…</code> will
            lose access immediately. This cannot be undone.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={revoke.isPending}>Cancel</AlertDialogCancel>
          <AlertDialogAction
            variant="destructive"
            onClick={confirm}
            disabled={revoke.isPending}
          >
            {revoke.isPending && <Spinner />}
            Revoke key
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
