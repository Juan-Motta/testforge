"use client";

import { LayersIcon, SquareIcon } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Spinner } from "@/components/ui/spinner";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { formatDateTime, formatRelative } from "@/lib/format";
import type { RunBatch } from "@/lib/api/types";
import { cn } from "@/lib/utils";

import { batchProgress } from "../lib/progress";

type BatchListProps = {
  batches: RunBatch[] | undefined;
  selected: string | null;
  canOperate: boolean;
  cancellingId: string | null;
  onSelect: (batchId: string | null) => void;
  onCancel: (batch: RunBatch) => void;
};

export function batchTitle(batch: Pick<RunBatch, "label" | "createdAt">): string {
  return batch.label || formatDateTime(batch.createdAt);
}

export function BatchList({ batches, selected, canOperate, cancellingId, onSelect, onCancel }: BatchListProps) {
  return (
    <nav aria-label="Batches" className="flex flex-col gap-1">
      <p className="px-2 pb-1 text-xs font-medium text-muted-foreground">Batches</p>
      <button
        type="button"
        onClick={() => onSelect(null)}
        aria-current={selected === null ? "true" : undefined}
        className={cn(
          "flex items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm outline-none hover:bg-muted focus-visible:ring-3 focus-visible:ring-ring/50",
          selected === null && "bg-muted font-medium",
        )}
      >
        <LayersIcon aria-hidden className="size-4 text-muted-foreground" />
        All runs
      </button>
      {batches === undefined && (
        <div className="flex flex-col gap-2 px-2 py-1" aria-busy aria-label="Loading batches">
          <Skeleton className="h-10 w-full" />
          <Skeleton className="h-10 w-full" />
        </div>
      )}
      {batches?.length === 0 && <p className="px-2 py-1 text-xs text-muted-foreground">No batches yet.</p>}
      {batches?.map((batch) => {
        const progress = batchProgress(batch);
        const isSelected = selected === batch.id;
        return (
          <div
            key={batch.id}
            className={cn(
              "group relative flex flex-col gap-1.5 rounded-md px-2 py-2 hover:bg-muted",
              isSelected && "bg-muted",
            )}
          >
            <div className="flex min-w-0 items-center gap-2">
              <button
                type="button"
                onClick={() => onSelect(batch.id)}
                aria-current={isSelected ? "true" : undefined}
                className={cn(
                  "min-w-0 flex-1 truncate text-left text-sm outline-none after:absolute after:inset-0 after:rounded-md focus-visible:after:ring-3 focus-visible:after:ring-ring/50",
                  isSelected && "font-medium",
                )}
                title={formatDateTime(batch.createdAt)}
              >
                {batchTitle(batch)}
              </button>
              {canOperate && !progress.done && (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon-xs"
                      className="relative z-10"
                      onClick={() => onCancel(batch)}
                      disabled={cancellingId === batch.id}
                      aria-label={`Cancel batch ${batchTitle(batch)}`}
                    >
                      {cancellingId === batch.id ? <Spinner /> : <SquareIcon />}
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>Cancel unfinished runs</TooltipContent>
                </Tooltip>
              )}
            </div>
            <div
              className="h-1 overflow-hidden rounded-full bg-muted-foreground/15"
              role="progressbar"
              aria-label={`${batchTitle(batch)} progress`}
              aria-valuemin={0}
              aria-valuemax={progress.total}
              aria-valuenow={progress.finished}
            >
              <div
                className={cn("h-full rounded-full transition-all", progress.failed > 0 ? "bg-warning" : "bg-brand")}
                style={{ width: `${progress.percent}%` }}
              />
            </div>
            <p className="flex justify-between gap-2 text-xs text-muted-foreground tabular-nums">
              <span>
                {progress.finished}/{progress.total} done{progress.failed > 0 ? ` · ${progress.failed} failed` : ""}
              </span>
              <span>{formatRelative(batch.createdAt)}</span>
            </p>
          </div>
        );
      })}
    </nav>
  );
}
