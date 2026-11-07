"use client";

import { get, useFormState } from "react-hook-form";

import type { AgentFormValues } from "../lib/form";
import { fieldId } from "./field-id";

/** Message the API attached to a whole list (e.g. `secrets`) rather than to one row. */
export function ListError({ name }: { name: string }) {
  const { errors } = useFormState<AgentFormValues>({ name: name as never });
  const message = (get(errors, name) as { message?: string } | undefined)?.message;
  if (!message) return null;
  return (
    <p id={fieldId(name)} role="alert" className="text-sm text-destructive">
      {message}
    </p>
  );
}
