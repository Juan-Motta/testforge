"use client";

import { PlayIcon, XIcon } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";

type SelectionBarProps = {
  selectedCount: number;
  /** Selected scenarios among the visible ones. */
  visibleSelected: number;
  visibleCount: number;
  onSelectVisible: (checked: boolean) => void;
  onClear: () => void;
  onRun: () => void;
};

/** Shown while scenarios are selected: select the visible ones, clear, or run them. */
export function SelectionBar({
  selectedCount,
  visibleSelected,
  visibleCount,
  onSelectVisible,
  onClear,
  onRun,
}: SelectionBarProps) {
  const all = visibleCount > 0 && visibleSelected === visibleCount;
  return (
    <div
      role="toolbar"
      aria-label="Selected scenarios"
      className="flex flex-wrap items-center gap-3 rounded-lg border bg-muted/30 px-5 py-2"
    >
      <Checkbox
        checked={all ? true : visibleSelected > 0 ? "indeterminate" : false}
        onCheckedChange={() => onSelectVisible(!all)}
        aria-label={all ? "Deselect visible scenarios" : "Select visible scenarios"}
      />
      <span className="text-sm font-medium tabular-nums">{selectedCount} selected</span>
      <div className="ml-auto flex items-center gap-2">
        <Button type="button" variant="ghost" size="sm" onClick={onClear}>
          <XIcon />
          Clear
        </Button>
        <Button type="button" size="sm" onClick={onRun}>
          <PlayIcon />
          Run selected
        </Button>
      </div>
    </div>
  );
}
