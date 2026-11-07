"use client";

import { Controller, useFormContext, type Path } from "react-hook-form";

import { Field, FieldDescription, FieldError, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";

import type { ScenarioFormValues } from "../lib/form";
import { fieldId } from "./field-id";

/** Client check for a whole number in range; the API validates again. */
export function wholeNumberRule(min: number, max: number) {
  return (value: string) => {
    const trimmed = value.trim();
    if (trimmed === "") return "Enter a number.";
    const number = Number(trimmed);
    if (!Number.isInteger(number)) return "Enter a whole number.";
    if (number < min || number > max) return `Use a value from ${min} to ${max}.`;
    return true;
  };
}

type NumberFieldProps = {
  name: Path<ScenarioFormValues>;
  label: string;
  description: string;
  min: number;
  max: number;
};

export function NumberField({ name, label, description, min, max }: NumberFieldProps) {
  const { control } = useFormContext<ScenarioFormValues>();
  return (
    <Controller
      name={name}
      control={control}
      rules={{ validate: (value) => wholeNumberRule(min, max)(value as string) }}
      render={({ field, fieldState }) => (
        <Field data-invalid={fieldState.invalid}>
          <FieldLabel htmlFor={fieldId(name)}>{label}</FieldLabel>
          <Input
            {...field}
            value={field.value as string}
            id={fieldId(name)}
            type="number"
            inputMode="numeric"
            min={min}
            max={max}
            aria-invalid={fieldState.invalid}
          />
          {fieldState.error ? <FieldError errors={[fieldState.error]} /> : <FieldDescription>{description}</FieldDescription>}
        </Field>
      )}
    />
  );
}
