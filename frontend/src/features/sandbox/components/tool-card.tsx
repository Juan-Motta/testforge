"use client";

import { XIcon } from "lucide-react";
import type { ReactNode } from "react";
import { Controller, useFormContext, type ControllerRenderProps, type FieldPath } from "react-hook-form";

import { Button } from "@/components/ui/button";
import { Field, FieldDescription, FieldError, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";

import { DEFAULT_ERROR_MESSAGE, MAX_LATENCY_MS, toolRules, type SandboxFormValues, type ToolFormValues } from "../lib/form";
import { fieldId } from "./field-id";

type ToolFieldProps = {
  index: number;
  name: keyof ToolFormValues;
  label: string;
  description?: ReactNode;
  validate?: (value: string) => true | string;
  children: (props: { id: string; invalid: boolean; field: ControllerRenderProps<SandboxFormValues> & { value: string } }) => ReactNode;
};

/** One labelled input of a tool, with its client rule and error. */
function ToolField({ index, name, label, description, validate, children }: ToolFieldProps) {
  const { control } = useFormContext<SandboxFormValues>();
  const path = `tools.${index}.${name}` as FieldPath<SandboxFormValues>;
  return (
    <Controller
      name={path}
      control={control}
      rules={validate ? { validate: (value) => validate(value as string) } : undefined}
      render={({ field, fieldState }) => (
        <Field data-invalid={fieldState.invalid}>
          <FieldLabel htmlFor={fieldId(path)}>{label}</FieldLabel>
          {children({
            id: fieldId(path),
            invalid: fieldState.invalid,
            field: { ...field, value: field.value as string },
          })}
          {fieldState.error ? (
            <FieldError errors={[fieldState.error]} />
          ) : description ? (
            <FieldDescription>{description}</FieldDescription>
          ) : null}
        </Field>
      )}
    />
  );
}

type ToolCardProps = {
  index: number;
  onRemove: () => void;
  readOnly: boolean;
};

export function ToolCard({ index, onRemove, readOnly }: ToolCardProps) {
  const { getValues } = useFormContext<SandboxFormValues>();
  const otherNames = () =>
    getValues("tools")
      .filter((_, position) => position !== index)
      .map((tool) => tool.name);

  return (
    <li className="flex flex-col gap-4 rounded-lg border p-4">
      <div className="flex items-start gap-2">
        <div className="grid min-w-0 flex-1 gap-4 @md:grid-cols-[14rem_minmax(0,1fr)]">
          <ToolField index={index} name="name" label="Name" validate={(value) => toolRules.name(value, otherNames())}>
            {({ id, invalid, field }) => (
              <Input
                {...field}
                id={id}
                aria-invalid={invalid}
                maxLength={64}
                placeholder="get_order_status"
                autoComplete="off"
                spellCheck={false}
                className="font-mono text-xs md:text-xs"
              />
            )}
          </ToolField>
          <ToolField index={index} name="description" label="Description" validate={toolRules.description}>
            {({ id, invalid, field }) => (
              <Input
                {...field}
                id={id}
                aria-invalid={invalid}
                maxLength={1000}
                placeholder="Looks up an order by its number."
                autoComplete="off"
              />
            )}
          </ToolField>
        </div>
        {!readOnly && (
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            className="mt-6 shrink-0"
            onClick={onRemove}
            aria-label={`Remove tool ${index + 1}`}
          >
            <XIcon />
          </Button>
        )}
      </div>
      <ToolField
        index={index}
        name="parameters"
        label="Parameters"
        description={
          <>
            JSON Schema of the arguments; its <code className="font-mono">type</code> must be{" "}
            <code className="font-mono">object</code>.
          </>
        }
        validate={toolRules.parameters}
      >
        {({ id, invalid, field }) => (
          <Textarea
            {...field}
            id={id}
            aria-invalid={invalid}
            spellCheck={false}
            className="min-h-28 font-mono text-xs md:text-xs"
          />
        )}
      </ToolField>
      <ToolField
        index={index}
        name="response"
        label="Mock response"
        description={
          <>
            Text returned to the model. <code className="font-mono">{"{{args.<name>}}"}</code> is replaced by that
            argument.
          </>
        }
      >
        {({ id, invalid, field }) => (
          <Textarea
            {...field}
            id={id}
            aria-invalid={invalid}
            placeholder={'{"orderId": "{{args.orderId}}", "status": "shipped"}'}
            className="min-h-20 font-mono text-xs md:text-xs"
          />
        )}
      </ToolField>
      <div className="grid gap-4 @md:grid-cols-[8rem_8rem_minmax(0,1fr)]">
        <ToolField index={index} name="latencyMs" label="Latency (ms)" validate={toolRules.latencyMs}>
          {({ id, invalid, field }) => (
            <Input
              {...field}
                id={id}
              aria-invalid={invalid}
              type="number"
              inputMode="numeric"
              min={0}
              max={MAX_LATENCY_MS}
              step={100}
            />
          )}
        </ToolField>
        <ToolField index={index} name="errorRate" label="Error rate (%)" validate={toolRules.errorRate}>
          {({ id, invalid, field }) => (
            <Input
              {...field}
                id={id}
              aria-invalid={invalid}
              type="number"
              inputMode="decimal"
              min={0}
              max={100}
              step={1}
            />
          )}
        </ToolField>
        <ToolField index={index} name="errorMessage" label="Error message">
          {({ id, invalid, field }) => (
            <Input
              {...field}
                id={id}
              aria-invalid={invalid}
              maxLength={500}
              placeholder={DEFAULT_ERROR_MESSAGE}
              autoComplete="off"
            />
          )}
        </ToolField>
      </div>
    </li>
  );
}
