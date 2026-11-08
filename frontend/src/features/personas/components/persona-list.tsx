"use client";

import { AudioLinesIcon, ClipboardListIcon, EllipsisIcon, GlobeIcon, PencilIcon, Trash2Icon, UserRoundIcon } from "lucide-react";

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
import type { Connection, Persona } from "@/lib/api/types";

import { TemperamentBadge } from "./temperament-badge";

type PersonaListProps = {
  personas: Persona[] | undefined;
  connections: Connection[] | undefined;
  loading: boolean;
  canAuthor: boolean;
  onEdit: (persona: Persona) => void;
  onDelete: (persona: Persona) => void;
};

function PersonaCardSkeleton() {
  return (
    <Card className="gap-4 p-5">
      <div className="flex items-center gap-3">
        <Skeleton className="size-9 rounded-lg" />
        <div className="flex flex-col gap-2">
          <Skeleton className="h-4 w-32" />
          <Skeleton className="h-3 w-20" />
        </div>
      </div>
      <Skeleton className="h-4 w-full" />
      <Skeleton className="h-4 w-40" />
    </Card>
  );
}

export function usedByLabel(count: number): string {
  if (count === 0) return "Not used yet";
  return count === 1 ? "Used by 1 scenario" : `Used by ${count} scenarios`;
}

export function PersonaList({ personas, connections, loading, canAuthor, onEdit, onDelete }: PersonaListProps) {
  if (loading) {
    return (
      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3" aria-busy aria-label="Loading personas">
        <PersonaCardSkeleton />
        <PersonaCardSkeleton />
        <PersonaCardSkeleton />
      </div>
    );
  }

  return (
    <ul className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
      {personas?.map((persona) => {
        const voiceConnection = persona.voice
          ? connections?.find((connection) => connection.id === persona.voice?.connectionId)
          : undefined;
        return (
          <li key={persona.id} className="flex">
            <Card className="relative w-full gap-4 p-5 transition-colors focus-within:ring-3 focus-within:ring-ring/50 hover:bg-muted/30">
              <div className="flex items-start gap-3">
                <span className="flex size-9 shrink-0 items-center justify-center rounded-lg border bg-background shadow-xs [&_svg]:size-4">
                  <UserRoundIcon aria-hidden />
                </span>
                <div className="flex min-w-0 flex-1 flex-col gap-1">
                  <h3 className="truncate text-sm font-medium">
                    {canAuthor ? (
                      <button
                        type="button"
                        className="text-left outline-none after:absolute after:inset-0"
                        onClick={() => onEdit(persona)}
                      >
                        {persona.name}
                      </button>
                    ) : (
                      persona.name
                    )}
                  </h3>
                  <div className="flex flex-wrap items-center gap-2">
                    <TemperamentBadge temperament={persona.temperament} />
                    <span className="inline-flex items-center gap-1 font-mono text-xs text-muted-foreground">
                      <GlobeIcon aria-hidden className="size-3" />
                      {persona.locale}
                    </span>
                  </div>
                </div>
                {canAuthor && (
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        className="relative z-10 -mt-1 -mr-2"
                        aria-label={`Actions for ${persona.name}`}
                      >
                        <EllipsisIcon />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      <DropdownMenuItem onSelect={() => onEdit(persona)}>
                        <PencilIcon />
                        Edit
                      </DropdownMenuItem>
                      <DropdownMenuSeparator />
                      <DropdownMenuItem variant="destructive" onSelect={() => onDelete(persona)}>
                        <Trash2Icon />
                        Delete
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                )}
              </div>
              {persona.notes && <p className="line-clamp-2 text-sm text-muted-foreground">{persona.notes}</p>}
              <div className="mt-auto flex flex-wrap items-center gap-x-4 gap-y-2 border-t pt-4 text-xs text-muted-foreground">
                <span className="inline-flex min-w-0 items-center gap-1">
                  <AudioLinesIcon aria-hidden className="size-3.5 shrink-0" />
                  <span className="truncate">
                    {persona.voice
                      ? `${voiceConnection?.name ?? "Connection"} · ${persona.voice.voice}`
                      : "No voice"}
                  </span>
                </span>
                <span className="inline-flex items-center gap-1">
                  <ClipboardListIcon aria-hidden className="size-3.5" />
                  {usedByLabel(persona.scenarioCount)}
                </span>
              </div>
            </Card>
          </li>
        );
      })}
    </ul>
  );
}
