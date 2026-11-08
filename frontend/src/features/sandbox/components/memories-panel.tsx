"use client";

import { BrainIcon, RotateCwIcon, Trash2Icon } from "lucide-react";
import { toast } from "sonner";

import { FormError } from "@/components/form-error";
import { Button } from "@/components/ui/button";
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Spinner } from "@/components/ui/spinner";
import { errorMessage } from "@/lib/api/errors";
import { formatDateTime, formatRelative } from "@/lib/format";

import { useDeleteSandboxMemory, useSandboxMemories } from "../api/sandbox";
import { DEFAULT_USER_ID } from "../lib/playground";

type MemoriesPanelProps = {
  slug: string;
  agentId: string;
  userId: string;
  canDelete: boolean;
};

export function MemoriesPanel({ slug, agentId, userId, canDelete }: MemoriesPanelProps) {
  const owner = userId.trim() || DEFAULT_USER_ID;
  const memories = useSandboxMemories(slug, agentId, owner);
  const remove = useDeleteSandboxMemory(slug, agentId);

  function deleteMemory(memoryId: string) {
    remove.mutate(memoryId, {
      onSuccess: () => toast.success("Memory deleted"),
      onError: (error) => toast.error(errorMessage(error)),
    });
  }

  return (
    <Card className="gap-4">
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <BrainIcon aria-hidden className="size-4 text-brand" />
          Memories
        </CardTitle>
        <CardDescription>
          Facts the agent remembered for <code className="font-mono">{owner}</code>.
        </CardDescription>
        <CardAction>
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            aria-label="Refresh memories"
            onClick={() => memories.refetch()}
            disabled={memories.isFetching}
          >
            {memories.isFetching ? <Spinner /> : <RotateCwIcon />}
          </Button>
        </CardAction>
      </CardHeader>
      <CardContent className="flex flex-col gap-2">
        {memories.isPending ? (
          <div className="flex flex-col gap-2" aria-busy aria-label="Loading memories">
            <Skeleton className="h-10" />
            <Skeleton className="h-10" />
          </div>
        ) : memories.isError ? (
          <FormError message={errorMessage(memories.error)} />
        ) : memories.data.length === 0 ? (
          <p className="text-sm text-muted-foreground">Nothing remembered yet.</p>
        ) : (
          <ul className="flex flex-col gap-2">
            {memories.data.map((memory) => (
              <li key={memory.id} className="flex items-start gap-2 rounded-md border px-3 py-2">
                <div className="flex min-w-0 flex-1 flex-col gap-0.5">
                  <p className="text-sm break-words">{memory.content}</p>
                  <time dateTime={memory.createdAt} title={formatDateTime(memory.createdAt)} className="text-xs text-muted-foreground">
                    {formatRelative(memory.createdAt)}
                  </time>
                </div>
                {canDelete && (
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-xs"
                    aria-label="Delete memory"
                    onClick={() => deleteMemory(memory.id)}
                    disabled={remove.isPending && remove.variables === memory.id}
                  >
                    <Trash2Icon />
                  </Button>
                )}
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}
