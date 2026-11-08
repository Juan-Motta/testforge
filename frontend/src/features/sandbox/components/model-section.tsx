"use client";

import Link from "next/link";
import { Controller, useFormContext } from "react-hook-form";

import { Field, FieldDescription, FieldError, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { EditorSection } from "@/features/agents/components/editor-section";
import { Segmented } from "@/features/scenarios/components/segmented";
import type { Connection } from "@/lib/api/types";

import { sandboxConnections, temperatureRule, type SandboxFormValues } from "../lib/form";
import { ConnectionSelect } from "./connection-select";
import { fieldId } from "./field-id";

const MODALITY_OPTIONS = [
  { value: "text" as const, label: "Text" },
  { value: "voice" as const, label: "Voice", disabledReason: "Voice sandbox agents are coming soon." },
];

type ModelSectionProps = {
  connections: Connection[] | undefined;
  connectionsHref: string;
  readOnly: boolean;
};

export function ModelSection({ connections, connectionsHref, readOnly }: ModelSectionProps) {
  const { control } = useFormContext<SandboxFormValues>();
  const options = sandboxConnections(connections ?? []);

  return (
    <EditorSection title="Model" description="The provider and model that answer as this agent." disabled={readOnly}>
      <Controller
        name="modality"
        control={control}
        render={({ field, fieldState }) => (
          <Field data-invalid={fieldState.invalid}>
            <FieldLabel htmlFor={fieldId("modality")}>Modality</FieldLabel>
            <Segmented
              id={fieldId("modality")}
              label="Modality"
              value={field.value}
              onChange={(value) => value === "text" && field.onChange(value)}
              options={MODALITY_OPTIONS}
              disabled={readOnly}
              invalid={fieldState.invalid}
            />
            {fieldState.error ? (
              <FieldError errors={[fieldState.error]} />
            ) : (
              <FieldDescription>Voice is coming soon.</FieldDescription>
            )}
          </Field>
        )}
      />
      <div className="grid gap-4 @md:grid-cols-2">
        <Controller
          name="connectionId"
          control={control}
          rules={{ validate: (value) => value !== "" || "Choose an OpenAI or Azure OpenAI connection." }}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor={fieldId("connectionId")}>Connection</FieldLabel>
              <ConnectionSelect
                id={fieldId("connectionId")}
                value={field.value}
                onChange={field.onChange}
                options={options}
                invalid={fieldState.invalid}
                disabled={readOnly}
              />
              {fieldState.error ? (
                <FieldError errors={[fieldState.error]} />
              ) : (
                <FieldDescription>
                  OpenAI or Azure OpenAI.{" "}
                  {options.length === 0 && (
                    <Link href={connectionsHref} className="underline underline-offset-2">
                      Add a connection
                    </Link>
                  )}
                </FieldDescription>
              )}
            </Field>
          )}
        />
        <Controller
          name="model"
          control={control}
          rules={{ validate: (value) => value.trim() !== "" || "Enter a model or deployment name." }}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor={fieldId("model")}>Model</FieldLabel>
              <Input
                {...field}
                id={fieldId("model")}
                maxLength={200}
                placeholder="gpt-5-mini"
                autoComplete="off"
                spellCheck={false}
                aria-invalid={fieldState.invalid}
                className="font-mono text-xs md:text-xs"
              />
              {fieldState.error ? (
                <FieldError errors={[fieldState.error]} />
              ) : (
                <FieldDescription>For Azure OpenAI, the deployment name.</FieldDescription>
              )}
            </Field>
          )}
        />
      </div>
      <Controller
        name="temperature"
        control={control}
        rules={{ validate: temperatureRule }}
        render={({ field, fieldState }) => (
          <Field data-invalid={fieldState.invalid} className="@md:max-w-48">
            <FieldLabel htmlFor={fieldId("temperature")}>Temperature</FieldLabel>
            <Input
              {...field}
              id={fieldId("temperature")}
              type="number"
              inputMode="decimal"
              min={0}
              max={2}
              step={0.1}
              placeholder="Default"
              aria-invalid={fieldState.invalid}
            />
            {fieldState.error ? (
              <FieldError errors={[fieldState.error]} />
            ) : (
              <FieldDescription>Optional, 0 to 2.</FieldDescription>
            )}
          </Field>
        )}
      />
    </EditorSection>
  );
}
