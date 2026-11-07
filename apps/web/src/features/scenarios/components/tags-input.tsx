"use client";

import { XIcon } from "lucide-react";
import { useState } from "react";

import { cn } from "@/lib/utils";

export const MAX_TAGS = 20;
export const MAX_TAG_LENGTH = 40;

type TagsInputProps = {
  id: string;
  value: string[];
  onChange: (tags: string[]) => void;
  onBlur?: () => void;
  disabled?: boolean;
  invalid?: boolean;
};

/** Tags as chips: Enter or comma adds the typed tag, Backspace on an empty input removes the last one. */
export function TagsInput({ id, value, onChange, onBlur, disabled, invalid }: TagsInputProps) {
  const [draft, setDraft] = useState("");
  const full = value.length >= MAX_TAGS;

  function add() {
    const tag = draft.trim();
    if (tag === "") return;
    if (!value.some((existing) => existing.toLowerCase() === tag.toLowerCase()) && !full) {
      onChange([...value, tag.slice(0, MAX_TAG_LENGTH)]);
    }
    setDraft("");
  }

  return (
    <div
      className={cn(
        "flex min-h-9 w-full flex-wrap items-center gap-1.5 rounded-lg border border-input bg-transparent px-2 py-1.5 text-sm shadow-xs transition-[color,box-shadow] focus-within:border-ring focus-within:ring-3 focus-within:ring-ring/50 dark:bg-input/30",
        invalid && "border-destructive ring-destructive/20",
        disabled && "opacity-50",
      )}
    >
      {value.map((tag) => (
        <span key={tag} className="inline-flex h-6 items-center gap-1 rounded-md bg-muted pr-1 pl-2 text-xs">
          {tag}
          {!disabled && (
            <button
              type="button"
              className="rounded-sm text-muted-foreground outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring/50"
              aria-label={`Remove tag ${tag}`}
              onClick={() => onChange(value.filter((existing) => existing !== tag))}
            >
              <XIcon className="size-3" />
            </button>
          )}
        </span>
      ))}
      <input
        id={id}
        value={draft}
        disabled={disabled || full}
        aria-invalid={invalid}
        maxLength={MAX_TAG_LENGTH}
        placeholder={full ? `Up to ${MAX_TAGS} tags` : value.length === 0 ? "Add a tag and press Enter" : "Add tag"}
        autoComplete="off"
        className="h-6 min-w-24 flex-1 bg-transparent outline-none placeholder:text-muted-foreground disabled:cursor-not-allowed"
        onChange={(event) => setDraft(event.target.value.replace(",", ""))}
        onKeyDown={(event) => {
          if (event.key === "Enter" || event.key === ",") {
            event.preventDefault();
            add();
          } else if (event.key === "Backspace" && draft === "" && value.length > 0) {
            onChange(value.slice(0, -1));
          }
        }}
        onBlur={() => {
          add();
          onBlur?.();
        }}
      />
    </div>
  );
}
