"use client";

import { PlusIcon } from "lucide-react";
import { Controller, useFormContext } from "react-hook-form";

import { Button } from "@/components/ui/button";
import { Field, FieldDescription, FieldError, FieldLabel } from "@/components/ui/field";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { EditorSection } from "@/features/agents/components/editor-section";
import { MODALITY_LABELS } from "@/features/agents/lib/display";
import type { Agent, Persona, ScenarioGroup } from "@/lib/api/types";

import type { ScenarioFormValues } from "../lib/form";
import { fieldId } from "./field-id";
import { TagsInput } from "./tags-input";

const NO_GROUP = "none";

type BasicsSectionProps = {
  agents: Agent[];
  personas: Persona[] | undefined;
  groups: ScenarioGroup[] | undefined;
  /** The agent cannot change once the scenario exists. */
  agentLocked: boolean;
  readOnly: boolean;
  onAgentChange: (agent: Agent) => void;
  onNewPersona: () => void;
};

export function BasicsSection({
  agents,
  personas,
  groups,
  agentLocked,
  readOnly,
  onAgentChange,
  onNewPersona,
}: BasicsSectionProps) {
  const { control } = useFormContext<ScenarioFormValues>();

  return (
    <EditorSection title="Basics" description="Who is tested, who calls, and where the scenario lives." disabled={readOnly}>
      <div className="grid gap-5 @lg:grid-cols-2">
        <Controller
          name="agentId"
          control={control}
          rules={{ validate: (value) => value !== "" || "Pick the agent this scenario tests." }}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor={fieldId("agentId")}>Agent</FieldLabel>
              <Select
                value={field.value}
                onValueChange={(id) => {
                  field.onChange(id);
                  const agent = agents.find((item) => item.id === id);
                  if (agent) onAgentChange(agent);
                }}
                disabled={agentLocked || readOnly}
              >
                <SelectTrigger id={fieldId("agentId")} className="w-full" aria-invalid={fieldState.invalid}>
                  <SelectValue placeholder="Pick an agent" />
                </SelectTrigger>
                <SelectContent>
                  {agents.map((agent) => (
                    <SelectItem key={agent.id} value={agent.id}>
                      {agent.name}
                      <span className="text-xs text-muted-foreground">{MODALITY_LABELS[agent.modality]}</span>
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {fieldState.error ? (
                <FieldError errors={[fieldState.error]} />
              ) : (
                <FieldDescription>
                  {agentLocked ? "A scenario keeps the agent it was created for." : "Can't be changed after creation."}
                </FieldDescription>
              )}
            </Field>
          )}
        />
        <Controller
          name="personaId"
          control={control}
          rules={{ validate: (value) => value !== "" || "Pick who talks to the agent." }}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <div className="flex items-center justify-between gap-2">
                <FieldLabel htmlFor={fieldId("personaId")}>Persona</FieldLabel>
                {!readOnly && (
                  <Button type="button" variant="ghost" size="xs" className="-my-1" onClick={onNewPersona}>
                    <PlusIcon />
                    New persona
                  </Button>
                )}
              </div>
              <Select value={field.value} onValueChange={field.onChange} disabled={readOnly}>
                <SelectTrigger id={fieldId("personaId")} className="w-full" aria-invalid={fieldState.invalid}>
                  <SelectValue placeholder={personas?.length === 0 ? "No personas yet" : "Pick a persona"} />
                </SelectTrigger>
                <SelectContent>
                  {personas?.map((persona) => (
                    <SelectItem key={persona.id} value={persona.id}>
                      {persona.name}
                      <span className="font-mono text-xs text-muted-foreground">{persona.locale}</span>
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {fieldState.error ? (
                <FieldError errors={[fieldState.error]} />
              ) : (
                <FieldDescription>The simulated user, reusable across scenarios.</FieldDescription>
              )}
            </Field>
          )}
        />
        <Controller
          name="groupId"
          control={control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor={fieldId("groupId")}>Group</FieldLabel>
              <Select
                value={field.value === "" ? NO_GROUP : field.value}
                onValueChange={(value) => field.onChange(value === NO_GROUP ? "" : value)}
                disabled={readOnly}
              >
                <SelectTrigger id={fieldId("groupId")} className="w-full" aria-invalid={fieldState.invalid}>
                  <SelectValue placeholder="No group" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={NO_GROUP}>No group</SelectItem>
                  {groups?.map((group) => (
                    <SelectItem key={group.id} value={group.id}>
                      {group.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {fieldState.error && <FieldError errors={[fieldState.error]} />}
            </Field>
          )}
        />
        <Controller
          name="tags"
          control={control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor={fieldId("tags")}>Tags</FieldLabel>
              <TagsInput
                id={fieldId("tags")}
                value={field.value}
                onChange={field.onChange}
                onBlur={field.onBlur}
                disabled={readOnly}
                invalid={fieldState.invalid}
              />
              {fieldState.error ? (
                <FieldError errors={[fieldState.error]} />
              ) : (
                <FieldDescription>Filter and pick scenarios by tag, e.g. billing or smoke.</FieldDescription>
              )}
            </Field>
          )}
        />
      </div>
    </EditorSection>
  );
}
