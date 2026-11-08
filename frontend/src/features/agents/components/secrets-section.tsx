"use client";

import { PlusIcon, TriangleAlertIcon, XIcon } from "lucide-react";
import { Controller, useFieldArray, useFormContext, useWatch } from "react-hook-form";

import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { FieldError } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { SecretInput } from "@/features/connections/components/secret-input";
import type { Agent } from "@/lib/api/types";

import type { AgentFormValues } from "../lib/form";
import { EditorSection } from "./editor-section";
import { fieldId } from "./field-id";
import { ListError } from "./list-error";

type SecretsSectionProps = {
  /** The stored agent when editing; secrets then show their hint and may be left empty. */
  stored?: Agent;
  readOnly: boolean;
};

export function SecretsSection({ stored, readOnly }: SecretsSectionProps) {
  const { control } = useFormContext<AgentFormValues>();
  const { fields, append, remove } = useFieldArray({ control, name: "secrets" });
  const url = useWatch({ control, name: "transport.url" });
  const preSessionEnabled = useWatch({ control, name: "transport.preSessionEnabled" });
  const preSessionUrl = useWatch({ control, name: "transport.preSession.url" });
  const secrets = useWatch({ control, name: "secrets" });

  const storedPreSessionUrl = stored?.transport.preSession?.url ?? null;
  const urlsChanged =
    stored !== undefined &&
    (url.trim() !== stored.transport.url ||
      (preSessionEnabled ? preSessionUrl.trim() : null) !== storedPreSessionUrl);
  const missingValues = secrets.some((secret) => secret.name.trim() !== "" && secret.value.trim() === "");

  return (
    <EditorSection
      title="Secrets"
      description={
        <>
          Encrypted at rest and never shown again. Reference them as{" "}
          <code className="font-mono">{"{{secret.<name>}}"}</code>.
        </>
      }
      actions={
        !readOnly && (
          <Button type="button" variant="outline" size="sm" onClick={() => append({ name: "", value: "" })}>
            <PlusIcon />
            Add secret
          </Button>
        )
      }
      disabled={readOnly}
    >
      {urlsChanged && missingValues && (
        <Alert>
          <TriangleAlertIcon />
          <AlertDescription>
            You changed the agent or pre-session URL. Enter every secret again so stored values only reach the
            hosts they were entered for.
          </AlertDescription>
        </Alert>
      )}
      <ListError name="secrets" />
      {fields.length === 0 ? (
        <p className="rounded-lg border border-dashed px-3 py-2.5 text-xs text-muted-foreground">
          No secrets. Add API keys or tokens the agent needs in headers, query parameters or bodies.
        </p>
      ) : (
        <div className="flex flex-col gap-3">
          {fields.map((row, index) => {
            const hint = secrets[index]?.hint;
            return (
              <div
                key={row.id}
                className="grid grid-cols-[minmax(0,1fr)_1.75rem] gap-2 sm:grid-cols-[minmax(0,2fr)_minmax(0,3fr)_1.75rem]"
              >
                <Controller
                  name={`secrets.${index}.name`}
                  control={control}
                  render={({ field, fieldState }) => (
                    <div className="flex min-w-0 flex-col gap-1">
                      <Input
                        {...field}
                        id={fieldId(`secrets.${index}.name`)}
                        aria-label={`Secret ${index + 1} name`}
                        aria-invalid={fieldState.invalid}
                        placeholder="apiKey"
                        autoComplete="off"
                        spellCheck={false}
                        className="font-mono text-xs md:text-xs"
                      />
                      {fieldState.error && <FieldError errors={[fieldState.error]} />}
                    </div>
                  )}
                />
                <Controller
                  name={`secrets.${index}.value`}
                  control={control}
                  render={({ field, fieldState }) => (
                    <div className="flex min-w-0 flex-col gap-1">
                      <SecretInput
                        {...field}
                        id={fieldId(`secrets.${index}.value`)}
                        aria-label={`Secret ${index + 1} value`}
                        aria-invalid={fieldState.invalid}
                        placeholder={hint ?? "Value"}
                        className="font-mono text-xs md:text-xs"
                      />
                      {fieldState.error ? (
                        <FieldError errors={[fieldState.error]} />
                      ) : hint ? (
                        <p className="text-xs text-muted-foreground">Stored ({hint}). Leave empty to keep.</p>
                      ) : null}
                    </div>
                  )}
                />
                {!readOnly && (
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    className="col-start-2 row-start-1 sm:col-start-3"
                    aria-label={`Remove secret ${index + 1}`}
                    onClick={() => remove(index)}
                  >
                    <XIcon />
                  </Button>
                )}
              </div>
            );
          })}
        </div>
      )}
    </EditorSection>
  );
}
