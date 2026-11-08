"use client";

import { Controller, useFormContext } from "react-hook-form";

import { Field, FieldContent, FieldDescription, FieldError, FieldLabel } from "@/components/ui/field";
import { Switch } from "@/components/ui/switch";
import { EditorSection } from "@/features/agents/components/editor-section";

import type { SandboxFormValues } from "../lib/form";
import { fieldId } from "./field-id";

const TOGGLES = [
  {
    name: "memory.conversation",
    label: "Conversation memory",
    description: "Keeps each thread's last 50 messages. Off: every message is answered on its own.",
  },
  {
    name: "memory.longTerm",
    label: "Long-term memory",
    description: "Gives the model remember and recall tools that keep facts per user across threads.",
  },
] as const;

export function MemorySection({ readOnly }: { readOnly: boolean }) {
  const { control } = useFormContext<SandboxFormValues>();
  return (
    <EditorSection title="Memory" disabled={readOnly}>
      {TOGGLES.map((toggle) => (
        <Controller
          key={toggle.name}
          name={toggle.name}
          control={control}
          render={({ field, fieldState }) => (
            <Field orientation="horizontal" data-invalid={fieldState.invalid}>
              <FieldContent>
                <FieldLabel htmlFor={fieldId(toggle.name)}>{toggle.label}</FieldLabel>
                <FieldDescription>{toggle.description}</FieldDescription>
                {fieldState.error && <FieldError errors={[fieldState.error]} />}
              </FieldContent>
              <Switch
                id={fieldId(toggle.name)}
                checked={field.value}
                onCheckedChange={field.onChange}
                onBlur={field.onBlur}
                aria-invalid={fieldState.invalid}
              />
            </Field>
          )}
        />
      ))}
    </EditorSection>
  );
}
