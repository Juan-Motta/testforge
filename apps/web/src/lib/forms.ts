import type { FieldValues, Path, UseFormSetError } from "react-hook-form";

import { errorMessage, isApiError } from "./api/errors";

/**
 * Maps an API error onto a react-hook-form instance: `error.fields` entries that
 * match a form field are shown inline, anything else becomes a root error.
 */
export function applyApiError<T extends FieldValues>(
  error: unknown,
  setError: UseFormSetError<T>,
  fieldNames: readonly Path<T>[],
): void {
  let matched = false;
  if (isApiError(error)) {
    for (const [field, message] of Object.entries(error.fields)) {
      if ((fieldNames as readonly string[]).includes(field)) {
        setError(field as Path<T>, { type: "server", message }, { shouldFocus: !matched });
        matched = true;
      }
    }
  }
  if (!matched) {
    setError("root.server" as Path<T>, { type: "server", message: errorMessage(error) });
  }
}
