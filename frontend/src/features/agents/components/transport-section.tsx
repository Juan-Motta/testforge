"use client";

import { ChevronRightIcon, PlusIcon, XIcon } from "lucide-react";
import { useState } from "react";
import { Controller, get, useFieldArray, useFormContext, useFormState, useWatch } from "react-hook-form";

import { Button } from "@/components/ui/button";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { Field, FieldContent, FieldDescription, FieldError, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";

import type { AgentFormValues } from "../lib/form";
import { EditorSection } from "./editor-section";
import { fieldId } from "./field-id";
import { KeyValueTable } from "./key-value-table";
import { ListError } from "./list-error";

const MONO_TEXTAREA = "min-h-24 font-mono text-xs md:text-xs";

function InitMessages({ readOnly }: { readOnly: boolean }) {
  const { control } = useFormContext<AgentFormValues>();
  const { fields, append, remove } = useFieldArray({ control, name: "transport.initMessages" });
  return (
    <div className="flex flex-col gap-2" role="group" aria-label="Initial messages">
      <div className="flex items-center justify-between gap-2">
        <span className="text-sm font-medium">Initial messages</span>
        {!readOnly && (
          <Button type="button" variant="ghost" size="xs" onClick={() => append({ value: "" })}>
            <PlusIcon />
            Add message
          </Button>
        )}
      </div>
      <p className="text-xs text-muted-foreground">JSON messages sent right after the WebSocket connects, in order.</p>
      <ListError name="transport.initMessages" />
      {fields.length === 0 ? (
        <p className="rounded-lg border border-dashed px-3 py-2.5 text-xs text-muted-foreground">None yet.</p>
      ) : (
        fields.map((row, index) => (
          <Controller
            key={row.id}
            name={`transport.initMessages.${index}.value`}
            control={control}
            render={({ field, fieldState }) => (
              <div className="flex gap-2">
                <div className="flex min-w-0 flex-1 flex-col gap-1">
                  <Textarea
                    {...field}
                    id={fieldId(`transport.initMessages.${index}.value`)}
                    aria-label={`Initial message ${index + 1}`}
                    aria-invalid={fieldState.invalid}
                    spellCheck={false}
                    placeholder='{"type": "session.update"}'
                    className={MONO_TEXTAREA}
                  />
                  {fieldState.error && <FieldError errors={[fieldState.error]} />}
                </div>
                {!readOnly && (
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    aria-label={`Remove initial message ${index + 1}`}
                    onClick={() => remove(index)}
                  >
                    <XIcon />
                  </Button>
                )}
              </div>
            )}
          />
        ))
      )}
    </div>
  );
}

function Handshake({ readOnly }: { readOnly: boolean }) {
  const { control } = useFormContext<AgentFormValues>();
  const enabled = useWatch({ control, name: "transport.preSessionEnabled" });
  const method = useWatch({ control, name: "transport.preSession.method" });
  const { errors } = useFormState({ control, name: "transport.preSession" });
  const hasError = get(errors, "transport.preSession") !== undefined;
  const [open, setOpen] = useState(enabled);

  return (
    <Collapsible open={open || hasError} onOpenChange={setOpen} className="rounded-lg border">
      <CollapsibleTrigger asChild>
        <button
          type="button"
          className="group flex w-full items-center gap-2 px-3 py-2.5 text-left text-sm font-medium outline-none focus-visible:ring-3 focus-visible:ring-ring/50"
        >
          <ChevronRightIcon className="size-4 text-muted-foreground transition-transform group-data-[state=open]:rotate-90" />
          Handshake
          <span className="ml-auto text-xs font-normal text-muted-foreground">
            {enabled ? `${method} pre-session call` : "Off"}
          </span>
        </button>
      </CollapsibleTrigger>
      <CollapsibleContent className="flex flex-col gap-5 border-t px-3 py-4">
        <Controller
          name="transport.preSessionEnabled"
          control={control}
          render={({ field }) => (
            <Field orientation="horizontal">
              <Switch
                id={fieldId("transport.preSessionEnabled")}
                checked={field.value}
                onCheckedChange={field.onChange}
                disabled={readOnly}
              />
              <FieldContent>
                <FieldLabel htmlFor={fieldId("transport.preSessionEnabled")}>
                  Call an HTTP endpoint before connecting
                </FieldLabel>
                <FieldDescription>
                  Fields of its JSON response become <code className="font-mono">{"{{session.<name>}}"}</code>.
                </FieldDescription>
              </FieldContent>
            </Field>
          )}
        />
        {enabled && (
          <>
            <div className="grid gap-3 sm:grid-cols-[7rem_minmax(0,1fr)]">
              <Controller
                name="transport.preSession.method"
                control={control}
                render={({ field }) => (
                  <Field>
                    <FieldLabel htmlFor={fieldId("transport.preSession.method")}>Method</FieldLabel>
                    <Select value={field.value} onValueChange={field.onChange} disabled={readOnly}>
                      <SelectTrigger id={fieldId("transport.preSession.method")} className="w-full">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="POST">POST</SelectItem>
                        <SelectItem value="GET">GET</SelectItem>
                      </SelectContent>
                    </Select>
                  </Field>
                )}
              />
              <Controller
                name="transport.preSession.url"
                control={control}
                rules={{
                  validate: (value, values) =>
                    !values.transport.preSessionEnabled ||
                    value.trim() !== "" ||
                    "Enter the URL of the pre-session call.",
                }}
                render={({ field, fieldState }) => (
                  <Field data-invalid={fieldState.invalid}>
                    <FieldLabel htmlFor={fieldId("transport.preSession.url")}>URL</FieldLabel>
                    <Input
                      {...field}
                      id={fieldId("transport.preSession.url")}
                      aria-invalid={fieldState.invalid}
                      placeholder="https://agent.example/sessions"
                      autoComplete="off"
                      spellCheck={false}
                      className="font-mono text-xs md:text-xs"
                    />
                    {fieldState.error && <FieldError errors={[fieldState.error]} />}
                  </Field>
                )}
              />
            </div>
            <KeyValueTable
              name="transport.preSession.headers"
              label="Pre-session headers"
              addLabel="Add header"
              readOnly={readOnly}
            />
            {method === "POST" && (
              <Controller
                name="transport.preSession.body"
                control={control}
                render={({ field, fieldState }) => (
                  <Field data-invalid={fieldState.invalid}>
                    <FieldLabel htmlFor={fieldId("transport.preSession.body")}>Body</FieldLabel>
                    <Textarea
                      {...field}
                      id={fieldId("transport.preSession.body")}
                      aria-invalid={fieldState.invalid}
                      spellCheck={false}
                      placeholder='{"agentId": "{{var.agentId}}"}'
                      className={MONO_TEXTAREA}
                    />
                    {fieldState.error && <FieldError errors={[fieldState.error]} />}
                  </Field>
                )}
              />
            )}
            <KeyValueTable
              name="transport.preSession.captures"
              label="Captures"
              columns={[
                { field: "name", label: "Name", placeholder: "sessionId" },
                { field: "path", label: "Response path", placeholder: "data.session_id" },
              ]}
              addLabel="Add capture"
              emptyText="Nothing captured. Add a name and the dot path of a response field."
              readOnly={readOnly}
            />
          </>
        )}
      </CollapsibleContent>
    </Collapsible>
  );
}

export function TransportSection({ readOnly }: { readOnly: boolean }) {
  const { control } = useFormContext<AgentFormValues>();
  const modality = useWatch({ control, name: "modality" });
  const text = modality === "text";

  return (
    <EditorSection
      title="Transport"
      description={
        text
          ? "The HTTP request TestForge sends for every message."
          : "The WebSocket TestForge opens for every conversation."
      }
      disabled={readOnly}
    >
      <Controller
        name="transport.url"
        control={control}
        rules={{ validate: (value) => value.trim() !== "" || "Enter the URL of the agent." }}
        render={({ field, fieldState }) => (
          <Field data-invalid={fieldState.invalid}>
            <FieldLabel htmlFor={fieldId("transport.url")}>URL</FieldLabel>
            <Input
              {...field}
              id={fieldId("transport.url")}
              aria-invalid={fieldState.invalid}
              placeholder={text ? "https://agent.example/chat" : "wss://agent.example/ws"}
              autoComplete="off"
              spellCheck={false}
              className="font-mono"
            />
            {fieldState.error ? (
              <FieldError errors={[fieldState.error]} />
            ) : (
              <FieldDescription>
                {text ? "https://" : "wss://"} with a literal host. The path may use templates; put query
                parameters below.
              </FieldDescription>
            )}
          </Field>
        )}
      />
      <KeyValueTable name="transport.query" label="Query parameters" addLabel="Add parameter" readOnly={readOnly} />
      <KeyValueTable name="transport.headers" label="Headers" addLabel="Add header" readOnly={readOnly} />
      {text ? (
        <Controller
          name="transport.body"
          control={control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor={fieldId("transport.body")}>Body</FieldLabel>
              <Textarea
                {...field}
                id={fieldId("transport.body")}
                aria-invalid={fieldState.invalid}
                spellCheck={false}
                placeholder={'{"message": "{{message}}", "threadId": "{{run.thread_id}}"}'}
                className={MONO_TEXTAREA}
              />
              {fieldState.error ? (
                <FieldError errors={[fieldState.error]} />
              ) : (
                <FieldDescription>
                  JSON template; it must reference <code className="font-mono">{"{{message}}"}</code>.
                </FieldDescription>
              )}
            </Field>
          )}
        />
      ) : (
        <InitMessages readOnly={readOnly} />
      )}
      <Handshake readOnly={readOnly} />
    </EditorSection>
  );
}
