"use client";

import { PlusIcon, SparklesIcon } from "lucide-react";
import { get, useFieldArray, useFormContext, useFormState } from "react-hook-form";

import { Button } from "@/components/ui/button";
import { EditorSection } from "@/features/agents/components/editor-section";

import { emptyTool, MAX_TOOLS, type SandboxFormValues } from "../lib/form";
import { fieldId } from "./field-id";
import { ToolCard } from "./tool-card";

/** Message the API attached to the whole tool list (e.g. too many tools). */
function ToolsError() {
  const { errors } = useFormState<SandboxFormValues>({ name: "tools" });
  const message = (get(errors, "tools.root") as { message?: string } | undefined)?.message;
  if (!message) return null;
  return (
    <p id={fieldId("tools")} role="alert" className="text-sm text-destructive">
      {message}
    </p>
  );
}

type ToolsSectionProps = {
  readOnly: boolean;
  /** Opens the "Generate tool" dialog; omitted for roles that cannot generate. */
  onGenerate?: () => void;
};

export function ToolsSection({ readOnly, onGenerate }: ToolsSectionProps) {
  const { control } = useFormContext<SandboxFormValues>();
  const { fields, append, remove } = useFieldArray({ control, name: "tools" });
  const full = fields.length >= MAX_TOOLS;

  return (
    <EditorSection
      title="Tools"
      description={`Mock tools the model can call, with simulated latency and failures. Up to ${MAX_TOOLS}.`}
      disabled={readOnly}
      actions={
        !readOnly && (
          <>
            {onGenerate && (
              <Button type="button" variant="outline" size="sm" onClick={onGenerate} disabled={full}>
                <SparklesIcon />
                Generate tool
              </Button>
            )}
            <Button type="button" variant="outline" size="sm" onClick={() => append(emptyTool())} disabled={full}>
              <PlusIcon />
              Add tool
            </Button>
          </>
        )
      }
    >
      <ToolsError />
      {fields.length === 0 ? (
        <p className="text-sm text-muted-foreground">No tools. The agent only answers from its system prompt.</p>
      ) : (
        <ol className="flex flex-col gap-3">
          {fields.map((row, index) => (
            <ToolCard key={row.id} index={index} onRemove={() => remove(index)} readOnly={readOnly} />
          ))}
        </ol>
      )}
      {full && !readOnly && <p className="text-xs text-muted-foreground">You reached the limit of {MAX_TOOLS} tools.</p>}
    </EditorSection>
  );
}
