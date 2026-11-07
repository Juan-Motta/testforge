"use client";

import { ChevronRightIcon, PlusIcon, XIcon } from "lucide-react";
import { useState } from "react";
import { Controller, useFieldArray, useFormContext, useFormState, type Path } from "react-hook-form";

import { Button } from "@/components/ui/button";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { Field, FieldDescription, FieldError, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { EditorSection } from "@/features/agents/components/editor-section";

import type { ScenarioFormValues } from "../lib/form";
import { fieldId } from "./field-id";
import { ListError } from "./list-error";

const MAX_FACTS = 50;

function TextAreaField({
  name,
  label,
  description,
  placeholder,
  maxLength,
  required,
}: {
  name: "goal" | "instructions" | "context";
  label: string;
  description: string;
  placeholder: string;
  maxLength: number;
  required?: string;
}) {
  const { control } = useFormContext<ScenarioFormValues>();
  return (
    <Controller
      name={name}
      control={control}
      rules={required ? { validate: (value) => value.trim() !== "" || required } : undefined}
      render={({ field, fieldState }) => (
        <Field data-invalid={fieldState.invalid}>
          <FieldLabel htmlFor={fieldId(name)}>{label}</FieldLabel>
          <Textarea
            {...field}
            id={fieldId(name)}
            aria-invalid={fieldState.invalid}
            maxLength={maxLength}
            placeholder={placeholder}
            className="min-h-20"
          />
          {fieldState.error ? <FieldError errors={[fieldState.error]} /> : <FieldDescription>{description}</FieldDescription>}
        </Field>
      )}
    />
  );
}

function FactsTable({ readOnly }: { readOnly: boolean }) {
  const { control } = useFormContext<ScenarioFormValues>();
  const { fields, append, remove } = useFieldArray({ control, name: "facts" });

  return (
    <div className="flex flex-col gap-2" role="group" aria-label="Facts">
      <div className="flex items-center justify-between gap-2">
        <div className="flex flex-col gap-0.5">
          <span className="text-sm font-medium">Facts</span>
          <span className="text-xs text-muted-foreground">Revealed only when the agent asks, e.g. an account number.</span>
        </div>
        {!readOnly && (
          <Button
            type="button"
            variant="ghost"
            size="xs"
            disabled={fields.length >= MAX_FACTS}
            onClick={() => append({ key: "", value: "" })}
          >
            <PlusIcon />
            Add fact
          </Button>
        )}
      </div>
      <ListError name="facts" />
      {fields.length === 0 ? (
        <p className="rounded-lg border border-dashed px-3 py-2.5 text-xs text-muted-foreground">No facts yet.</p>
      ) : (
        <div className="flex flex-col gap-2">
          <div className="grid grid-cols-[minmax(0,2fr)_minmax(0,3fr)_1.75rem] gap-2 px-0.5 text-xs text-muted-foreground max-sm:hidden">
            <span>Key</span>
            <span>Value</span>
          </div>
          {fields.map((row, index) => (
            <div
              key={row.id}
              className="grid grid-cols-[minmax(0,1fr)_1.75rem] gap-2 sm:grid-cols-[minmax(0,2fr)_minmax(0,3fr)_1.75rem]"
            >
              {(["key", "value"] as const).map((column) => {
                const path = `facts.${index}.${column}` as Path<ScenarioFormValues>;
                return (
                  <Controller
                    key={column}
                    name={path}
                    control={control}
                    render={({ field, fieldState }) => (
                      <div className={column === "value" ? "flex min-w-0 flex-col gap-1 max-sm:col-start-1" : "flex min-w-0 flex-col gap-1"}>
                        <Input
                          {...field}
                          value={field.value as string}
                          id={fieldId(path)}
                          aria-label={`Fact ${index + 1} ${column}`}
                          aria-invalid={fieldState.invalid}
                          placeholder={column === "key" ? "account_number" : "88231"}
                          maxLength={column === "key" ? 80 : 500}
                          autoComplete="off"
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
                  className="col-start-2 row-start-1 sm:col-start-3"
                  aria-label={`Remove fact ${index + 1}`}
                  onClick={() => remove(index)}
                >
                  <XIcon />
                </Button>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

export function ConversationSection({ readOnly, contextOpen }: { readOnly: boolean; contextOpen: boolean }) {
  const [open, setOpen] = useState(contextOpen);
  // A server error on the context opens it so the message is visible.
  const { errors } = useFormState<ScenarioFormValues>({ name: "context" });
  return (
    <EditorSection
      title="Conversation"
      description="What the simulated user wants and how they behave."
      disabled={readOnly}
    >
      <TextAreaField
        name="goal"
        label="Goal"
        description="What the user is trying to get done. The judge checks whether it happened."
        placeholder="Get the late fee on the March invoice refunded."
        maxLength={5000}
        required="Describe what the simulated user wants."
      />
      <TextAreaField
        name="instructions"
        label="Instructions"
        description="How the simulated user behaves during the conversation."
        placeholder="Mention the fee only after greeting. If offered a credit instead of a refund, refuse once."
        maxLength={10000}
      />
      <Collapsible open={open || errors.context !== undefined} onOpenChange={setOpen} className="flex flex-col gap-3">
        <CollapsibleTrigger className="group inline-flex w-fit items-center gap-1.5 text-sm font-medium outline-none focus-visible:ring-3 focus-visible:ring-ring/50">
          <ChevronRightIcon aria-hidden className="size-4 transition-transform group-data-[state=open]:rotate-90" />
          Context
        </CollapsibleTrigger>
        <CollapsibleContent>
          <TextAreaField
            name="context"
            label="Background"
            description="Anything else the simulated user knows: history with the company, recent events."
            placeholder="Customer since 2019, paid every invoice on time until March."
            maxLength={20000}
          />
        </CollapsibleContent>
      </Collapsible>
      <FactsTable readOnly={readOnly} />
    </EditorSection>
  );
}
