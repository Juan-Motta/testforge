"use client";

import { ArrowDownIcon, ArrowUpIcon, CircleAlertIcon, CircleCheckIcon, PlusIcon, ScaleIcon, XIcon } from "lucide-react";
import { Controller, useFieldArray, useFormContext, useWatch, type Path } from "react-hook-form";

import { Button } from "@/components/ui/button";
import { FieldError } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { EditorSection } from "@/features/agents/components/editor-section";
import type { CriterionSource } from "@/lib/api/types";
import { cn } from "@/lib/utils";

import { CRITERION_SOURCES, distributeEvenly, weightTotal, type SourceAvailability } from "../lib/criteria";
import { emptyCriterion, type ScenarioFormValues } from "../lib/form";
import { fieldId } from "./field-id";
import { ListError } from "./list-error";
import { wholeNumberRule } from "./number-field";
import { Segmented } from "./segmented";

export const MAX_CRITERIA = 30;

const KIND_OPTIONS = [
  { value: "must" as const, label: "Must" },
  { value: "should" as const, label: "Should" },
];

type CriteriaSectionProps = {
  sources: Record<CriterionSource, SourceAvailability>;
  readOnly: boolean;
};

export function CriteriaSection({ sources, readOnly }: CriteriaSectionProps) {
  const { control, setValue } = useFormContext<ScenarioFormValues>();
  const { fields, append, remove, move } = useFieldArray({ control, name: "criteria" });
  const criteria = useWatch({ control, name: "criteria" });
  const total = weightTotal(criteria.map((criterion) => criterion.weight));
  const sourceOptions = CRITERION_SOURCES.map((source) => ({
    value: source.value,
    label: source.label,
    disabledReason: sources[source.value].available ? undefined : sources[source.value].reason,
  }));

  function distribute() {
    distributeEvenly(criteria.length).forEach((weight, index) =>
      setValue(`criteria.${index}.weight`, String(weight), { shouldDirty: true, shouldValidate: true }),
    );
  }

  return (
    <EditorSection
      title="Criteria"
      description="What a good conversation must or should include. The judge scores each one; weights total 100."
      disabled={readOnly}
      actions={
        !readOnly && (
          <Button type="button" variant="outline" size="sm" onClick={distribute} disabled={criteria.length === 0}>
            <ScaleIcon />
            Distribute evenly
          </Button>
        )
      }
    >
      <ListError name="criteria" />
      <ol className="flex flex-col gap-3">
        {fields.map((row, index) => (
          <li key={row.id} className="flex flex-col gap-3 rounded-lg border p-3">
            <div className="flex items-start gap-2">
              <span className="mt-2 w-5 shrink-0 text-right text-xs text-muted-foreground tabular-nums">{index + 1}.</span>
              <Controller
                name={`criteria.${index}.text`}
                control={control}
                rules={{ validate: (value) => value.trim() !== "" || "Describe what the judge checks." }}
                render={({ field, fieldState }) => (
                  <div className="flex min-w-0 flex-1 flex-col gap-1">
                    <Input
                      {...field}
                      id={fieldId(`criteria.${index}.text`)}
                      aria-label={`Criterion ${index + 1}`}
                      aria-invalid={fieldState.invalid}
                      maxLength={500}
                      placeholder="Verifies the caller's identity before discussing the account"
                      autoComplete="off"
                    />
                    {fieldState.error && <FieldError errors={[fieldState.error]} />}
                  </div>
                )}
              />
              {!readOnly && (
                <div className="flex shrink-0 items-center">
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    aria-label={`Move criterion ${index + 1} up`}
                    disabled={index === 0}
                    onClick={() => move(index, index - 1)}
                  >
                    <ArrowUpIcon />
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    aria-label={`Move criterion ${index + 1} down`}
                    disabled={index === fields.length - 1}
                    onClick={() => move(index, index + 1)}
                  >
                    <ArrowDownIcon />
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    aria-label={`Remove criterion ${index + 1}`}
                    disabled={fields.length === 1}
                    onClick={() => remove(index)}
                  >
                    <XIcon />
                  </Button>
                </div>
              )}
            </div>
            <div className="flex flex-wrap items-start gap-x-5 gap-y-3 pl-7">
              <Controller
                name={`criteria.${index}.kind`}
                control={control}
                render={({ field, fieldState }) => (
                  <div className="flex flex-col gap-1">
                    <span className="text-xs text-muted-foreground">Kind</span>
                    <Segmented
                      id={fieldId(`criteria.${index}.kind`)}
                      label={`Criterion ${index + 1} kind`}
                      value={field.value}
                      onChange={field.onChange}
                      options={KIND_OPTIONS}
                      disabled={readOnly}
                      invalid={fieldState.invalid}
                    />
                    {fieldState.error && <FieldError errors={[fieldState.error]} />}
                  </div>
                )}
              />
              <Controller
                name={`criteria.${index}.source`}
                control={control}
                render={({ field, fieldState }) => (
                  <div className="flex flex-col gap-1">
                    <span className="text-xs text-muted-foreground">Evidence</span>
                    <Segmented
                      id={fieldId(`criteria.${index}.source`)}
                      label={`Criterion ${index + 1} evidence`}
                      value={field.value}
                      onChange={field.onChange}
                      options={sourceOptions}
                      disabled={readOnly}
                      invalid={fieldState.invalid}
                    />
                    {fieldState.error && <FieldError errors={[fieldState.error]} />}
                  </div>
                )}
              />
              <Controller
                name={`criteria.${index}.weight` as Path<ScenarioFormValues>}
                control={control}
                rules={{ validate: (value) => wholeNumberRule(1, 100)(value as string) }}
                render={({ field, fieldState }) => (
                  <div className="flex flex-col gap-1">
                    <label htmlFor={fieldId(`criteria.${index}.weight`)} className="text-xs text-muted-foreground">
                      Weight
                    </label>
                    <Input
                      {...field}
                      value={field.value as string}
                      id={fieldId(`criteria.${index}.weight`)}
                      type="number"
                      inputMode="numeric"
                      min={1}
                      max={100}
                      aria-invalid={fieldState.invalid}
                      className="h-7 w-20 tabular-nums"
                    />
                    {fieldState.error && <FieldError errors={[fieldState.error]} />}
                  </div>
                )}
              />
            </div>
          </li>
        ))}
      </ol>
      <div className="flex flex-wrap items-center justify-between gap-3">
        {!readOnly ? (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            disabled={fields.length >= MAX_CRITERIA}
            onClick={() => append(emptyCriterion(total.total < 100 ? String(100 - total.total) : ""))}
          >
            <PlusIcon />
            Add criterion
          </Button>
        ) : (
          <span />
        )}
        <p
          role="status"
          aria-live="polite"
          className={cn(
            "inline-flex items-center gap-1.5 text-sm tabular-nums",
            total.status === "ok" ? "text-success" : "text-destructive",
          )}
        >
          {total.status === "ok" ? (
            <CircleCheckIcon aria-hidden className="size-4" />
          ) : (
            <CircleAlertIcon aria-hidden className="size-4" />
          )}
          <span className="font-medium">{total.total} / 100</span>
          <span className={total.status === "ok" ? "sr-only" : "text-muted-foreground"}>{total.message}</span>
        </p>
      </div>
    </EditorSection>
  );
}
