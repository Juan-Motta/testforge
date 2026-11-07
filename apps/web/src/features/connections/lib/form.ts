import { z } from "zod";

import type {
  Connection,
  ConnectionField,
  ConnectionKind,
  CreateConnectionRequest,
  UpdateConnectionRequest,
} from "@/lib/api/types";

/** Values of the add/edit form. `config` is keyed by the kind's field keys. */
export type ConnectionFormValues = {
  name: string;
  secret: string;
  config: Record<string, string>;
};

export type ConnectionFormMode = "create" | "edit";

/** Form path of a config field, as used by react-hook-form and the API's `error.fields`. */
export const configPath = (key: string) => `config.${key}` as const;

export function initialFormValues(kind: ConnectionKind, connection?: Connection): ConnectionFormValues {
  const config: Record<string, string> = {};
  for (const field of kind.fields) {
    config[field.key] = connection ? (connection.config[field.key] ?? "") : (field.default ?? "");
  }
  return { name: connection?.name ?? kind.label, secret: "", config };
}

/** "Endpoint" -> "endpoint", but acronyms such as "API key" stay as they are. */
function lowerFirst(value: string): string {
  if (/^[A-Z]{2}/.test(value)) return value;
  return value.charAt(0).toLowerCase() + value.slice(1);
}

function isHttpUrl(value: string): boolean {
  try {
    const url = new URL(value);
    return url.protocol === "https:" || url.protocol === "http:";
  } catch {
    return false;
  }
}

function fieldSchema(field: ConnectionField) {
  return z
    .string()
    .trim()
    .superRefine((value, ctx) => {
      if (value === "") {
        if (field.required && !field.default) {
          ctx.addIssue({ code: "custom", message: `Enter the ${lowerFirst(field.label)}.` });
        }
        return;
      }
      if (field.type === "url" && !isHttpUrl(value)) {
        ctx.addIssue({
          code: "custom",
          message: field.placeholder
            ? `Enter a valid URL, e.g. ${field.placeholder}.`
            : "Enter a valid URL.",
        });
        return;
      }
      if (field.pattern && !new RegExp(field.pattern).test(value)) {
        ctx.addIssue({ code: "custom", message: `${field.label} has an invalid format.` });
      }
    });
}

/** Client-side validation generated from the catalog; mirrors `CreateConnectionRequest`. */
export function buildConnectionSchema(kind: ConnectionKind, mode: ConnectionFormMode) {
  const secret = z.string().trim().max(4096, "Use 4096 characters or fewer.");
  return z.object({
    name: z
      .string()
      .trim()
      .min(1, "Enter a name for the connection.")
      .max(100, "Use 100 characters or fewer."),
    secret:
      mode === "create"
        ? secret.min(1, `Enter the ${lowerFirst(kind.secretLabel)}.`)
        : secret,
    config: z.object(Object.fromEntries(kind.fields.map((field) => [field.key, fieldSchema(field)]))),
  });
}

/** Trimmed config: empty values take the field default, or are left out. */
function buildConfig(kind: ConnectionKind, values: ConnectionFormValues): Record<string, string> {
  const config: Record<string, string> = {};
  for (const field of kind.fields) {
    const value = (values.config[field.key] ?? "").trim() || (field.default ?? "");
    if (value !== "") config[field.key] = value;
  }
  return config;
}

export function buildCreateRequest(
  kind: ConnectionKind,
  values: ConnectionFormValues,
): CreateConnectionRequest {
  return {
    name: values.name.trim(),
    kind: kind.kind,
    config: buildConfig(kind, values),
    secret: values.secret.trim(),
  };
}

/** `config` replaces the stored config; an empty secret is omitted to keep the stored one. */
export function buildUpdateRequest(
  kind: ConnectionKind,
  values: ConnectionFormValues,
): UpdateConnectionRequest {
  const body: UpdateConnectionRequest = {
    name: values.name.trim(),
    config: buildConfig(kind, values),
  };
  const secret = values.secret.trim();
  if (secret !== "") body.secret = secret;
  return body;
}

export type FormFieldPath = "name" | "secret" | `config.${string}`;

/**
 * Splits the API's `error.fields` into errors that belong to an input of this form
 * (`name`, `secret`, `config.<key>`) and messages that have no input to show on.
 */
export function mapServerFieldErrors(
  kind: ConnectionKind,
  fields: Record<string, string>,
): { matched: [FormFieldPath, string][]; unmatched: string[] } {
  const known = new Set<string>(["name", "secret", ...kind.fields.map((f) => configPath(f.key))]);
  const matched: [FormFieldPath, string][] = [];
  const unmatched: string[] = [];
  for (const [field, message] of Object.entries(fields)) {
    if (known.has(field)) matched.push([field as FormFieldPath, message]);
    else unmatched.push(message);
  }
  return { matched, unmatched };
}
