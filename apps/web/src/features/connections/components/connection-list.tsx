"use client";

import { EllipsisIcon, PencilIcon, Trash2Icon, ZapIcon } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Skeleton } from "@/components/ui/skeleton";
import type { Connection, ConnectionKind } from "@/lib/api/types";

import { CapabilityBadges, KindIcon } from "./capability-badges";
import { LastTestStatus } from "./last-test-status";

type ConnectionListProps = {
  connections: Connection[] | undefined;
  kinds: ConnectionKind[] | undefined;
  loading: boolean;
  canTest: boolean;
  canManage: boolean;
  isTesting: (id: string) => boolean;
  onTest: (connection: Connection) => void;
  onEdit: (connection: Connection) => void;
  onDelete: (connection: Connection) => void;
};

function ConnectionCardSkeleton() {
  return (
    <Card className="gap-4 p-5">
      <div className="flex items-center gap-3">
        <Skeleton className="size-9 rounded-lg" />
        <div className="flex flex-col gap-2">
          <Skeleton className="h-4 w-32" />
          <Skeleton className="h-3 w-20" />
        </div>
      </div>
      <Skeleton className="h-5 w-40" />
      <Skeleton className="h-4 w-48" />
    </Card>
  );
}

export function ConnectionList({
  connections,
  kinds,
  loading,
  canTest,
  canManage,
  isTesting,
  onTest,
  onEdit,
  onDelete,
}: ConnectionListProps) {
  if (loading) {
    return (
      <div className="grid gap-4 md:grid-cols-2" aria-busy aria-label="Loading connections">
        <ConnectionCardSkeleton />
        <ConnectionCardSkeleton />
      </div>
    );
  }

  return (
    <ul className="grid gap-4 md:grid-cols-2">
      {connections?.map((connection) => {
        const kindLabel = kinds?.find((kind) => kind.kind === connection.kind)?.label ?? connection.kind;
        const testing = isTesting(connection.id);
        return (
          <li key={connection.id} className="flex">
            <Card className="w-full gap-4 p-5">
              <div className="flex items-start gap-3">
                <KindIcon capabilities={connection.capabilities} />
                <div className="flex min-w-0 flex-1 flex-col gap-0.5">
                  <h3 className="truncate text-sm font-medium">{connection.name}</h3>
                  <p className="truncate text-xs text-muted-foreground">
                    {kindLabel} · Added by {connection.createdBy?.name ?? "Deleted user"}
                  </p>
                </div>
                {(canTest || canManage) && (
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        className="-mt-1 -mr-2"
                        aria-label={`Actions for ${connection.name}`}
                      >
                        <EllipsisIcon />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      {canTest && (
                        <DropdownMenuItem disabled={testing} onSelect={() => onTest(connection)}>
                          <ZapIcon />
                          Test connection
                        </DropdownMenuItem>
                      )}
                      {canManage && (
                        <>
                          <DropdownMenuItem onSelect={() => onEdit(connection)}>
                            <PencilIcon />
                            Edit
                          </DropdownMenuItem>
                          <DropdownMenuSeparator />
                          <DropdownMenuItem variant="destructive" onSelect={() => onDelete(connection)}>
                            <Trash2Icon />
                            Delete
                          </DropdownMenuItem>
                        </>
                      )}
                    </DropdownMenuContent>
                  </DropdownMenu>
                )}
              </div>
              <CapabilityBadges capabilities={connection.capabilities} />
              <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 border-t pt-4">
                <LastTestStatus lastTest={connection.lastTest} testing={testing} className="flex-1" />
                <code
                  className="rounded-md bg-muted px-1.5 py-0.5 font-mono text-xs text-muted-foreground"
                  aria-label="Secret hint"
                >
                  {connection.secretHint}
                </code>
              </div>
            </Card>
          </li>
        );
      })}
    </ul>
  );
}
