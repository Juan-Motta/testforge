"use client";

import { Controller, useFormContext } from "react-hook-form";

import { Field, FieldDescription, FieldError, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import type { Connection } from "@/lib/api/types";

import { DEFAULT_TRACE_SESSION_ID, type AgentFormValues } from "../lib/form";
import { EditorSection } from "./editor-section";
import { fieldId } from "./field-id";

// Radix Select does not accept an empty value, so "no connection" has its own token.
const NONE = "none";

type TracesSectionProps = {
  connections: Connection[] | undefined;
  readOnly: boolean;
};

export function TracesSection({ connections, readOnly }: TracesSectionProps) {
  const { control } = useFormContext<AgentFormValues>();
  const traceConnections = connections?.filter((connection) => connection.capabilities.includes("traces")) ?? [];

  return (
    <EditorSection
      title="Traces"
      description="Pull the agent's Langfuse traces into run reports."
      disabled={readOnly}
    >
      <Controller
        name="traceConnectionId"
        control={control}
        render={({ field, fieldState }) => (
          <Field data-invalid={fieldState.invalid}>
            <FieldLabel htmlFor={fieldId("traceConnectionId")}>Langfuse connection</FieldLabel>
            <Select
              value={field.value === "" ? NONE : field.value}
              onValueChange={(value) => field.onChange(value === NONE ? "" : value)}
              disabled={readOnly}
            >
              <SelectTrigger id={fieldId("traceConnectionId")} className="w-full" aria-invalid={fieldState.invalid}>
                <SelectValue placeholder="None" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={NONE}>None</SelectItem>
                {traceConnections.map((connection) => (
                  <SelectItem key={connection.id} value={connection.id}>
                    {connection.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {fieldState.error ? (
              <FieldError errors={[fieldState.error]} />
            ) : connections !== undefined && traceConnections.length === 0 ? (
              <FieldDescription>Add a Langfuse connection in Connections to pick it here.</FieldDescription>
            ) : null}
          </Field>
        )}
      />
      <Controller
        name="traceSessionId"
        control={control}
        render={({ field, fieldState }) => (
          <Field data-invalid={fieldState.invalid}>
            <FieldLabel htmlFor={fieldId("traceSessionId")}>Trace session id</FieldLabel>
            <Input
              {...field}
              id={fieldId("traceSessionId")}
              aria-invalid={fieldState.invalid}
              placeholder={DEFAULT_TRACE_SESSION_ID}
              autoComplete="off"
              spellCheck={false}
              className="font-mono text-xs md:text-xs"
            />
            {fieldState.error ? (
              <FieldError errors={[fieldState.error]} />
            ) : (
              <FieldDescription>Your agent should use this value as its Langfuse session id.</FieldDescription>
            )}
          </Field>
        )}
      />
    </EditorSection>
  );
}
