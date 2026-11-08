"use client";

import { Controller, useFormContext, useWatch } from "react-hook-form";

import { Field, FieldDescription, FieldError, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { EditorSection } from "@/features/agents/components/editor-section";
import type { Connection, Modality } from "@/lib/api/types";

import { simulatorCapability } from "../lib/criteria";
import type { ScenarioFormValues, VoiceSimulatorMode } from "../lib/form";
import { ConnectionSelect } from "./connection-select";
import { fieldId } from "./field-id";
import { Segmented } from "./segmented";

const MODE_OPTIONS: { value: VoiceSimulatorMode; label: string }[] = [
  { value: "pipeline", label: "Pipeline" },
  { value: "realtime", label: "Realtime" },
];

const MODE_DESCRIPTIONS: Record<VoiceSimulatorMode | "text", string> = {
  pipeline: "An LLM writes each line and text to speech says it.",
  realtime: "A speech-to-speech model talks directly.",
  text: "Text agents get a text simulator.",
};

function ModelInput({ name, placeholder }: { name: "simulator.model" | "judge.model"; placeholder: string }) {
  const { control } = useFormContext<ScenarioFormValues>();
  return (
    <Controller
      name={name}
      control={control}
      render={({ field, fieldState }) => (
        <Field data-invalid={fieldState.invalid}>
          <FieldLabel htmlFor={fieldId(name)}>Model</FieldLabel>
          <Input
            {...field}
            id={fieldId(name)}
            maxLength={100}
            placeholder={placeholder}
            autoComplete="off"
            spellCheck={false}
            aria-invalid={fieldState.invalid}
            className="font-mono text-xs md:text-xs"
          />
          {fieldState.error && <FieldError errors={[fieldState.error]} />}
        </Field>
      )}
    />
  );
}

type ModelsSectionProps = {
  modality: Modality | undefined;
  connections: Connection[] | undefined;
  readOnly: boolean;
};

export function ModelsSection({ modality, connections, readOnly }: ModelsSectionProps) {
  const { control, getValues, setValue } = useFormContext<ScenarioFormValues>();
  const mode = useWatch({ control, name: "simulator.mode" });
  const voice = modality !== "text";
  const capability = simulatorCapability(modality ?? "voice", mode);

  function changeMode(next: VoiceSimulatorMode) {
    setValue("simulator.mode", next, { shouldDirty: true });
    // The connection must match the new mode's capability.
    const connection = connections?.find((item) => item.id === getValues("simulator.connectionId"));
    if (connection && !connection.capabilities.includes(simulatorCapability("voice", next))) {
      setValue("simulator.connectionId", "", { shouldDirty: true });
    }
  }

  return (
    <EditorSection
      title="Simulator and judge"
      description="Optional for now: runs will ask for them when they need a model."
      disabled={readOnly}
    >
      <div className="flex flex-col gap-4">
        <span className="text-sm font-medium">Simulated user</span>
        <Controller
          name="simulator.mode"
          control={control}
          render={({ fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor={fieldId("simulator.mode")}>Mode</FieldLabel>
              {voice ? (
                <Segmented
                  id={fieldId("simulator.mode")}
                  label="Simulator mode"
                  value={mode}
                  onChange={changeMode}
                  options={MODE_OPTIONS}
                  disabled={readOnly}
                  invalid={fieldState.invalid}
                />
              ) : (
                <p id={fieldId("simulator.mode")} className="text-sm">
                  Text
                </p>
              )}
              {fieldState.error ? (
                <FieldError errors={[fieldState.error]} />
              ) : (
                <FieldDescription>{MODE_DESCRIPTIONS[voice ? mode : "text"]}</FieldDescription>
              )}
            </Field>
          )}
        />
        <Controller
          name="simulator.connectionId"
          control={control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor={fieldId("simulator.connectionId")}>Connection</FieldLabel>
              <ConnectionSelect
                id={fieldId("simulator.connectionId")}
                value={field.value}
                onChange={field.onChange}
                connections={connections}
                capability={capability}
                invalid={fieldState.invalid}
                disabled={readOnly}
              />
              {fieldState.error ? (
                <FieldError errors={[fieldState.error]} />
              ) : (
                <FieldDescription>{capability === "realtime" ? "Realtime connections." : "LLM connections."}</FieldDescription>
              )}
            </Field>
          )}
        />
        <ModelInput name="simulator.model" placeholder={capability === "realtime" ? "gpt-realtime" : "gpt-5-mini"} />
      </div>
      <div className="flex flex-col gap-4 border-t pt-5">
        <span className="text-sm font-medium">Judge</span>
        <Controller
          name="judge.connectionId"
          control={control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor={fieldId("judge.connectionId")}>Connection</FieldLabel>
              <ConnectionSelect
                id={fieldId("judge.connectionId")}
                value={field.value}
                onChange={field.onChange}
                connections={connections}
                capability="llm"
                invalid={fieldState.invalid}
                disabled={readOnly}
              />
              {fieldState.error ? (
                <FieldError errors={[fieldState.error]} />
              ) : (
                <FieldDescription>The LLM that scores the criteria.</FieldDescription>
              )}
            </Field>
          )}
        />
        <ModelInput name="judge.model" placeholder="gpt-5" />
      </div>
    </EditorSection>
  );
}
