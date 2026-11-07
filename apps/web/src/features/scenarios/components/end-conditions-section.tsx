"use client";

import { PlusIcon, XIcon } from "lucide-react";
import { Controller, useFieldArray, useFormContext, type Path } from "react-hook-form";

import { Button } from "@/components/ui/button";
import { Field, FieldDescription, FieldError, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { EditorSection } from "@/features/agents/components/editor-section";
import type { HangUpWhen, Modality } from "@/lib/api/types";

import type { ScenarioFormValues } from "../lib/form";
import { fieldId } from "./field-id";
import { ListError } from "./list-error";
import { NumberField } from "./number-field";

const MAX_INTERRUPTIONS = 10;

export const HANG_UP_OPTIONS: { value: HangUpWhen; label: string }[] = [
  { value: "goal_or_goodbye", label: "When the goal is met or someone says goodbye" },
  { value: "goodbye", label: "Only when someone says goodbye" },
  { value: "never", label: "Never, run until a limit is reached" },
];

export function EndConditionsSection({ modality, readOnly }: { modality: Modality | undefined; readOnly: boolean }) {
  const { control } = useFormContext<ScenarioFormValues>();
  return (
    <EditorSection title="End conditions" description="When the simulated user stops the conversation." disabled={readOnly}>
      <div className="grid gap-5 @lg:grid-cols-2">
        <NumberField
          name="endConditions.maxDurationSeconds"
          label="Max duration (seconds)"
          description="From 30 to 1800. Default 360."
          min={30}
          max={1800}
        />
        {modality === "text" ? (
          <NumberField name="endConditions.maxTurns" label="Max turns" description="From 1 to 100. Default 20." min={1} max={100} />
        ) : (
          <NumberField
            name="endConditions.maxSilenceSeconds"
            label="Max silence (seconds)"
            description="From 5 to 120. Default 15."
            min={5}
            max={120}
          />
        )}
      </div>
      <Controller
        name="endConditions.hangUpWhen"
        control={control}
        render={({ field, fieldState }) => (
          <Field data-invalid={fieldState.invalid}>
            <FieldLabel htmlFor={fieldId("endConditions.hangUpWhen")}>Hang up</FieldLabel>
            <Select value={field.value} onValueChange={field.onChange} disabled={readOnly}>
              <SelectTrigger id={fieldId("endConditions.hangUpWhen")} className="w-full" aria-invalid={fieldState.invalid}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {HANG_UP_OPTIONS.map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    {option.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {fieldState.error && <FieldError errors={[fieldState.error]} />}
          </Field>
        )}
      />
    </EditorSection>
  );
}

/** Agent replies the simulated user cuts off (voice agents only). */
export function InterruptionsSection({ readOnly }: { readOnly: boolean }) {
  const { control } = useFormContext<ScenarioFormValues>();
  const { fields, append, remove } = useFieldArray({ control, name: "interruptions" });

  return (
    <EditorSection
      title="Interruptions"
      description="Have the simulated user talk over the agent. Needs a codec that supports interruptions."
      disabled={readOnly}
      actions={
        !readOnly && (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            disabled={fields.length >= MAX_INTERRUPTIONS}
            onClick={() => append({ agentTurn: String(fields.length + 1), afterMs: "1000" })}
          >
            <PlusIcon />
            Add interruption
          </Button>
        )
      }
    >
      <ListError name="interruptions" />
      {fields.length === 0 ? (
        <p className="rounded-lg border border-dashed px-3 py-2.5 text-xs text-muted-foreground">
          No interruptions. The simulated user waits for each reply to finish.
        </p>
      ) : (
        <div className="flex flex-col gap-2">
          <div className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_1.75rem] gap-2 px-0.5 text-xs text-muted-foreground">
            <span>Agent reply #</span>
            <span>Interrupt after (ms)</span>
          </div>
          {fields.map((row, index) => (
            <div key={row.id} className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_1.75rem] gap-2">
              {(
                [
                  { column: "agentTurn", label: "agent reply", min: 1, max: undefined },
                  { column: "afterMs", label: "after milliseconds", min: 300, max: 60000 },
                ] as const
              ).map(({ column, label, min, max }) => {
                const path = `interruptions.${index}.${column}` as Path<ScenarioFormValues>;
                return (
                  <Controller
                    key={column}
                    name={path}
                    control={control}
                    render={({ field, fieldState }) => (
                      <div className="flex min-w-0 flex-col gap-1">
                        <Input
                          {...field}
                          value={field.value as string}
                          id={fieldId(path)}
                          type="number"
                          inputMode="numeric"
                          min={min}
                          max={max}
                          aria-label={`Interruption ${index + 1} ${label}`}
                          aria-invalid={fieldState.invalid}
                        />
                        {fieldState.error && <FieldError errors={[fieldState.error]} />}
                      </div>
                    )}
                  />
                );
              })}
              {!readOnly && (
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  aria-label={`Remove interruption ${index + 1}`}
                  onClick={() => remove(index)}
                >
                  <XIcon />
                </Button>
              )}
            </div>
          ))}
          <FieldDescription>From 300 to 60000 ms after the agent starts that reply.</FieldDescription>
        </div>
      )}
    </EditorSection>
  );
}
