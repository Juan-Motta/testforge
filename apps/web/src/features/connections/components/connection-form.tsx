"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { ArrowLeftIcon, ExternalLinkIcon } from "lucide-react";
import { Controller, useForm, type Path, type Resolver } from "react-hook-form";

import { FormError } from "@/components/form-error";
import { Button } from "@/components/ui/button";
import { DialogClose, DialogFooter } from "@/components/ui/dialog";
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import { errorMessage, isApiError } from "@/lib/api/errors";
import type { Connection, ConnectionKind } from "@/lib/api/types";

import {
  buildConnectionSchema,
  configPath,
  initialFormValues,
  mapServerFieldErrors,
  type ConnectionFormMode,
  type ConnectionFormValues,
} from "../lib/form";
import { SecretInput } from "./secret-input";

type ConnectionFormProps = {
  kind: ConnectionKind;
  mode: ConnectionFormMode;
  connection?: Connection;
  submitLabel: string;
  onBack?: () => void;
  /** Sends the request; a rejection is shown on the form. */
  onSubmit: (values: ConnectionFormValues) => Promise<unknown>;
};

function RequiredMark() {
  return (
    <span aria-hidden className="text-muted-foreground">
      *
    </span>
  );
}

export function ConnectionForm({
  kind,
  mode,
  connection,
  submitLabel,
  onBack,
  onSubmit,
}: ConnectionFormProps) {
  const form = useForm<ConnectionFormValues>({
    resolver: zodResolver(buildConnectionSchema(kind, mode)) as Resolver<ConnectionFormValues>,
    defaultValues: initialFormValues(kind, connection),
  });

  const submit = form.handleSubmit((values) =>
    onSubmit(values).catch((error: unknown) => {
      const { matched, unmatched } = mapServerFieldErrors(kind, isApiError(error) ? error.fields : {});
      matched.forEach(([field, message], index) =>
        form.setError(field as Path<ConnectionFormValues>, { type: "server", message }, { shouldFocus: index === 0 }),
      );
      if (matched.length === 0 || unmatched.length > 0) {
        form.setError("root.server", {
          type: "server",
          message: unmatched.length > 0 ? unmatched.join(" ") : errorMessage(error),
        });
      }
    }),
  );

  const idPrefix = `connection-${kind.kind}`;

  return (
    <form onSubmit={submit} noValidate className="flex flex-col gap-6">
      <FieldGroup>
        <FormError message={form.formState.errors.root?.server?.message} />
        <Controller
          name="name"
          control={form.control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor={`${idPrefix}-name`}>
                Name <RequiredMark />
              </FieldLabel>
              <Input
                {...field}
                id={`${idPrefix}-name`}
                autoComplete="off"
                aria-invalid={fieldState.invalid}
                aria-required
                autoFocus
              />
              {fieldState.invalid ? (
                <FieldError errors={[fieldState.error]} />
              ) : (
                <FieldDescription>Shown wherever you pick this connection, e.g. “OpenAI production”.</FieldDescription>
              )}
            </Field>
          )}
        />
        {kind.fields.map((configField) => {
          const required = configField.required && !configField.default;
          const id = `${idPrefix}-${configField.key}`;
          return (
            <Controller
              key={configField.key}
              name={configPath(configField.key)}
              control={form.control}
              render={({ field, fieldState }) => (
                <Field data-invalid={fieldState.invalid}>
                  <FieldLabel htmlFor={id}>
                    {configField.label} {required && <RequiredMark />}
                  </FieldLabel>
                  <Input
                    {...field}
                    value={field.value ?? ""}
                    id={id}
                    type={configField.type === "url" ? "url" : "text"}
                    inputMode={configField.type === "url" ? "url" : undefined}
                    placeholder={configField.placeholder ?? undefined}
                    autoComplete="off"
                    spellCheck={false}
                    aria-invalid={fieldState.invalid}
                    aria-required={required}
                  />
                  {fieldState.invalid ? (
                    <FieldError errors={[fieldState.error]} />
                  ) : configField.help ? (
                    <FieldDescription>{configField.help}</FieldDescription>
                  ) : null}
                </Field>
              )}
            />
          );
        })}
        <Controller
          name="secret"
          control={form.control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <div className="flex items-center justify-between gap-2">
                <FieldLabel htmlFor={`${idPrefix}-secret`}>
                  {kind.secretLabel} {mode === "create" && <RequiredMark />}
                </FieldLabel>
                <a
                  href={kind.docsUrl}
                  target="_blank"
                  rel="noreferrer"
                  className="inline-flex items-center gap-1 text-xs text-muted-foreground underline-offset-4 hover:text-foreground hover:underline"
                >
                  Where do I find this?
                  <ExternalLinkIcon aria-hidden className="size-3" />
                </a>
              </div>
              <SecretInput
                {...field}
                id={`${idPrefix}-secret`}
                aria-invalid={fieldState.invalid}
                aria-required={mode === "create"}
                placeholder={mode === "edit" ? "••••••••" : undefined}
              />
              {fieldState.invalid ? (
                <FieldError errors={[fieldState.error]} />
              ) : (
                <FieldDescription>
                  {mode === "edit" && connection
                    ? `Leave empty to keep the current secret (${connection.secretHint}).`
                    : "Encrypted at rest and never shown again; only its last 4 characters are kept visible."}
                </FieldDescription>
              )}
            </Field>
          )}
        />
      </FieldGroup>
      <DialogFooter className="sm:justify-between">
        {onBack ? (
          <Button type="button" variant="ghost" onClick={onBack} disabled={form.formState.isSubmitting}>
            <ArrowLeftIcon />
            Back
          </Button>
        ) : (
          <span />
        )}
        <div className="flex flex-col-reverse gap-2 sm:flex-row">
          <DialogClose asChild>
            <Button type="button" variant="outline">
              Cancel
            </Button>
          </DialogClose>
          <Button type="submit" disabled={form.formState.isSubmitting}>
            {form.formState.isSubmitting && <Spinner />}
            {submitLabel}
          </Button>
        </div>
      </DialogFooter>
    </form>
  );
}
