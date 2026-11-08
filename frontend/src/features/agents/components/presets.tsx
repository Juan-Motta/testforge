"use client";

import { LayersIcon } from "lucide-react";
import { useState } from "react";

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
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Skeleton } from "@/components/ui/skeleton";
import type { AgentPreset, Modality } from "@/lib/api/types";

import { ModalityBadge } from "./modality-badge";

type PresetGridProps = {
  presets: AgentPreset[] | undefined;
  onApply: (preset: AgentPreset) => void;
};

/** Preset cards shown on a new agent. Applying one only fills the form. */
export function PresetGrid({ presets, onApply }: PresetGridProps) {
  return (
    <section aria-labelledby="agent-presets-title" className="flex flex-col gap-3">
      <div className="flex flex-col gap-0.5">
        <h2 id="agent-presets-title" className="text-sm font-medium">
          Start from a preset
        </h2>
        <p className="text-sm text-muted-foreground">
          Fills the transport, codec and variables for a known protocol. You can change everything afterwards.
        </p>
      </div>
      <ul className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {presets === undefined
          ? [0, 1, 2].map((index) => (
              <li key={index}>
                <Skeleton className="h-24 rounded-xl" />
              </li>
            ))
          : presets.map((preset) => (
              <li key={preset.id} className="flex">
                <button
                  type="button"
                  onClick={() => onApply(preset)}
                  className="flex w-full flex-col gap-2 rounded-xl border bg-card p-4 text-left shadow-xs transition-colors outline-none hover:border-brand/60 hover:bg-muted/40 focus-visible:ring-3 focus-visible:ring-ring/50"
                >
                  <span className="flex items-center justify-between gap-2">
                    <span className="text-sm font-medium">{preset.label}</span>
                    <ModalityBadge modality={preset.modality} />
                  </span>
                  <span className="line-clamp-2 text-xs text-muted-foreground">{preset.description}</span>
                </button>
              </li>
            ))}
      </ul>
    </section>
  );
}

type PresetMenuProps = {
  presets: AgentPreset[] | undefined;
  modality: Modality;
  onApply: (preset: AgentPreset) => void;
};

/** On an existing agent: pick a preset of the same modality, after a confirmation. */
export function PresetMenu({ presets, modality, onApply }: PresetMenuProps) {
  const [pending, setPending] = useState<AgentPreset | null>(null);
  const available = presets?.filter((preset) => preset.modality === modality) ?? [];

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button type="button" variant="outline" disabled={available.length === 0}>
            <LayersIcon />
            <span className="max-sm:sr-only">Start from a preset</span>
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="w-64">
          <DropdownMenuLabel>Presets</DropdownMenuLabel>
          {available.map((preset) => (
            <DropdownMenuItem key={preset.id} onSelect={() => setPending(preset)} className="flex-col items-start gap-0.5">
              <span>{preset.label}</span>
              <span className="text-xs text-muted-foreground">{preset.description}</span>
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
      <AlertDialog open={pending !== null} onOpenChange={(open) => !open && setPending(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Apply “{pending?.label}”?</AlertDialogTitle>
            <AlertDialogDescription>
              It overwrites the transport, codec, variables and trace session id in the form, and adds any secrets
              it needs. Nothing is saved until you click Save.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                if (pending) onApply(pending);
                setPending(null);
              }}
            >
              Apply preset
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
