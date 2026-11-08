"use client";

import { Controller, useFormContext } from "react-hook-form";

import { Field, FieldDescription, FieldError, FieldLabel } from "@/components/ui/field";
import { Textarea } from "@/components/ui/textarea";
import { EditorSection } from "@/features/agents/components/editor-section";

import type { SandboxFormValues } from "../lib/form";
import { fieldId } from "./field-id";

export function InstructionsSection({ readOnly }: { readOnly: boolean }) {
  const { control } = useFormContext<SandboxFormValues>();
  return (
    <EditorSection title="Instructions" description="What the agent is for and how it behaves." disabled={readOnly}>
      <Controller
        name="description"
        control={control}
        render={({ field, fieldState }) => (
          <Field data-invalid={fieldState.invalid}>
            <FieldLabel htmlFor={fieldId("description")}>Description</FieldLabel>
            <Textarea
              {...field}
              id={fieldId("description")}
              aria-invalid={fieldState.invalid}
              maxLength={2000}
              placeholder="A pizza ordering assistant that checks the menu and places orders."
              className="min-h-20"
            />
            {fieldState.error ? (
              <FieldError errors={[fieldState.error]} />
            ) : (
              <FieldDescription>Optional. For your team; the model does not read it.</FieldDescription>
            )}
          </Field>
        )}
      />
      <Controller
        name="systemPrompt"
        control={control}
        rules={{ validate: (value) => value.trim() !== "" || "Write the system prompt." }}
        render={({ field, fieldState }) => (
          <Field data-invalid={fieldState.invalid}>
            <FieldLabel htmlFor={fieldId("systemPrompt")}>System prompt</FieldLabel>
            <Textarea
              {...field}
              id={fieldId("systemPrompt")}
              aria-invalid={fieldState.invalid}
              maxLength={20000}
              placeholder="You are Tony, the assistant of Tony's Pizza. Help customers order…"
              className="min-h-48 font-mono text-xs md:text-xs"
            />
            {fieldState.error && <FieldError errors={[fieldState.error]} />}
          </Field>
        )}
      />
    </EditorSection>
  );
}
