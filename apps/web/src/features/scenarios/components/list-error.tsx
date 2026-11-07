"use client";

import { get, useFormState } from "react-hook-form";

import type { ScenarioFormValues } from "../lib/form";
import { fieldId } from "./field-id";

/** Message the API attached to a whole list (e.g. `criteria`) rather than to one row. */
export function ListError({ name }: { name: "facts" | "criteria" | "interruptions" | "tags" }) {
  const { errors } = useFormState<ScenarioFormValues>({ name });
  // The editor stores list-level errors under `<list>.root` so row errors survive next to them.
  const message =
    (get(errors, `${name}.root`) as { message?: string } | undefined)?.message ??
    (get(errors, name) as { message?: string } | undefined)?.message;
  if (!message) return null;
  return (
    <p id={fieldId(name)} role="alert" className="text-sm text-destructive">
      {message}
    </p>
  );
}
