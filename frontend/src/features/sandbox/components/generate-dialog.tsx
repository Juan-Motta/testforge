"use client";

import { SparklesIcon } from "lucide-react";
import { Controller, useForm } from "react-hook-form";

import { FormError } from "@/components/form-error";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import { Textarea } from "@/components/ui/textarea";
import { applyApiError } from "@/lib/forms";
import type { Connection, SandboxDraft, SandboxGenerateContext, SandboxGenerateRequest } from "@/lib/api/types";

import { useGenerateSandboxDraft } from "../api/sandbox";
import { llmConnections } from "../lib/form";
import { ConnectionSelect } from "./connection-select";

type GenerateKind = SandboxGenerateRequest["kind"];

type GenerateValues = { connectionId: string; model: string; description: string };
const FIELD_NAMES = ["connectionId", "model", "description"] as const;

const COPY: Record<GenerateKind, { title: string; description: string; placeholder: string; submit: string }> = {
  agent: {
    title: "Generate agent with AI",
    description:
      "Describe the agent. The name, description, system prompt, memory and tools in the form are replaced; nothing is saved until you save.",
    placeholder: "A support agent for an online shoe store that can look up orders, start returns and check stock.",
    submit: "Generate agent",
  },
  tool: {
    title: "Generate tool with AI",
    description: "Describe the tool. It is added to the list with its schema and mock response; nothing is saved until you save.",
    placeholder: "Looks up an order by its number and returns its status and delivery date.",
    submit: "Generate tool",
  },
};

type GenerateDialogProps = {
  slug: string;
  kind: GenerateKind;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  connections: Connection[] | undefined;
  /** Prefills the connection and model (e.g. the agent's own when it is an LLM). */
  defaults: { connectionId: string; model: string };
  /** The current draft sent as context (kind `tool`). */
  context?: () => SandboxGenerateContext;
  onDraft: (draft: SandboxDraft) => void;
};

export function GenerateDialog({ open, onOpenChange, kind, ...props }: GenerateDialogProps) {
  const copy = COPY[kind];
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{copy.title}</DialogTitle>
          <DialogDescription>{copy.description}</DialogDescription>
        </DialogHeader>
        {/* Mounted only while open, so every visit starts from the current defaults. */}
        {open && (
          <GenerateForm
            {...props}
            kind={kind}
            onDraft={(draft) => {
              onOpenChange(false);
              props.onDraft(draft);
            }}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

function GenerateForm({ slug, kind, connections, defaults, context, onDraft }: Omit<GenerateDialogProps, "open" | "onOpenChange">) {
  const generate = useGenerateSandboxDraft(slug);
  const options = llmConnections(connections ?? []);
  const copy = COPY[kind];
  const defaultConnection = options.some((connection) => connection.id === defaults.connectionId) ? defaults.connectionId : "";
  const form = useForm<GenerateValues>({
    defaultValues: { connectionId: defaultConnection, model: defaultConnection ? defaults.model : "", description: "" },
  });
  const { control, formState } = form;

  const submit = form.handleSubmit(async (values) => {
    const body: SandboxGenerateRequest = {
      kind,
      connectionId: values.connectionId,
      model: values.model.trim(),
      description: values.description.trim(),
    };
    if (context) body.agent = context();
    try {
      onDraft(await generate.mutateAsync(body));
    } catch (error) {
      applyApiError(error, form.setError, FIELD_NAMES);
    }
  });

  return (
    <form onSubmit={submit} noValidate className="flex flex-col gap-6">
      <FieldGroup>
        <FormError message={formState.errors.root?.server?.message} />
        <div className="grid gap-4 sm:grid-cols-2">
          <Controller
            name="connectionId"
            control={control}
            rules={{ validate: (value) => value !== "" || "Choose an LLM connection." }}
            render={({ field, fieldState }) => (
              <Field data-invalid={fieldState.invalid}>
                <FieldLabel htmlFor="generate-connection">Connection</FieldLabel>
                <ConnectionSelect
                  id="generate-connection"
                  value={field.value}
                  onChange={field.onChange}
                  options={options}
                  invalid={fieldState.invalid}
                />
                {fieldState.error ? <FieldError errors={[fieldState.error]} /> : <FieldDescription>Any LLM connection.</FieldDescription>}
              </Field>
            )}
          />
          <Controller
            name="model"
            control={control}
            rules={{ validate: (value) => value.trim() !== "" || "Enter a model." }}
            render={({ field, fieldState }) => (
              <Field data-invalid={fieldState.invalid}>
                <FieldLabel htmlFor="generate-model">Model</FieldLabel>
                <Input
                  {...field}
                  id="generate-model"
                  maxLength={200}
                  placeholder="gpt-5"
                  autoComplete="off"
                  spellCheck={false}
                  aria-invalid={fieldState.invalid}
                  className="font-mono text-xs md:text-xs"
                />
                {fieldState.error && <FieldError errors={[fieldState.error]} />}
              </Field>
            )}
          />
        </div>
        <Controller
          name="description"
          control={control}
          rules={{ validate: (value) => value.trim() !== "" || "Describe what to generate." }}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor="generate-description">Description</FieldLabel>
              <Textarea
                {...field}
                id="generate-description"
                maxLength={4000}
                placeholder={copy.placeholder}
                aria-invalid={fieldState.invalid}
                className="min-h-32"
                autoFocus
              />
              {fieldState.error && <FieldError errors={[fieldState.error]} />}
            </Field>
          )}
        />
      </FieldGroup>
      <DialogFooter>
        <DialogClose asChild>
          <Button type="button" variant="outline" disabled={formState.isSubmitting}>
            Cancel
          </Button>
        </DialogClose>
        <Button type="submit" disabled={formState.isSubmitting}>
          {formState.isSubmitting ? <Spinner /> : <SparklesIcon />}
          {formState.isSubmitting ? "Generating…" : copy.submit}
        </Button>
      </DialogFooter>
    </form>
  );
}
