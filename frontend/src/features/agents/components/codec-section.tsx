"use client";

import { Controller, useFormContext, useWatch } from "react-hook-form";

import { Field, FieldContent, FieldDescription, FieldError, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import type { AgentCodecSpec, CodecField, CodecType } from "@/lib/api/types";
import { cn } from "@/lib/utils";

import type { AgentFormValues } from "../lib/form";
import { EditorSection } from "./editor-section";
import { fieldId } from "./field-id";

function OptionField({ option, readOnly }: { option: CodecField; readOnly: boolean }) {
  const { control } = useFormContext<AgentFormValues>();
  const path = `codec.options.${option.key}` as const;
  const id = fieldId(path);
  const placeholder = option.placeholder ?? option.default ?? undefined;

  return (
    <Controller
      name={path}
      control={control}
      render={({ field, fieldState }) => {
        const value = field.value ?? "";
        const description = fieldState.error ? (
          <FieldError errors={[fieldState.error]} />
        ) : option.help ? (
          <FieldDescription>{option.help}</FieldDescription>
        ) : null;

        if (option.type === "boolean") {
          // Values are strings in the API; an unset option shows its default.
          const checked = (value || option.default || "false") === "true";
          return (
            <Field orientation="horizontal" data-invalid={fieldState.invalid}>
              <Switch
                id={id}
                checked={checked}
                onCheckedChange={(next) => field.onChange(next ? "true" : "false")}
                disabled={readOnly}
                aria-invalid={fieldState.invalid}
              />
              <FieldContent>
                <FieldLabel htmlFor={id}>{option.label}</FieldLabel>
                {description}
              </FieldContent>
            </Field>
          );
        }

        return (
          <Field data-invalid={fieldState.invalid}>
            <FieldLabel htmlFor={id}>
              {option.label}
              {option.required && !option.default && (
                <span aria-hidden className="text-muted-foreground">
                  *
                </span>
              )}
            </FieldLabel>
            {option.type === "json" ? (
              <Textarea
                {...field}
                value={value}
                id={id}
                aria-invalid={fieldState.invalid}
                placeholder={placeholder}
                spellCheck={false}
                className="min-h-20 font-mono text-xs md:text-xs"
              />
            ) : (
              <Input
                {...field}
                value={value}
                id={id}
                inputMode={option.type === "number" ? "numeric" : undefined}
                aria-invalid={fieldState.invalid}
                placeholder={placeholder}
                autoComplete="off"
                spellCheck={false}
                className={cn(option.type === "text" && "font-mono text-xs md:text-xs")}
              />
            )}
            {description}
          </Field>
        );
      }}
    />
  );
}

type CodecSectionProps = {
  codecs: AgentCodecSpec[] | undefined;
  readOnly: boolean;
};

export function CodecSection({ codecs, readOnly }: CodecSectionProps) {
  const { control, setValue } = useFormContext<AgentFormValues>();
  const modality = useWatch({ control, name: "modality" });
  const selected = useWatch({ control, name: "codec.type" });
  const available = codecs?.filter((codec) => codec.modality === modality);
  const spec = codecs?.find((codec) => codec.codec === selected);

  function choose(type: CodecType) {
    if (type === selected) return;
    setValue("codec.type", type, { shouldDirty: true, shouldValidate: true });
    // Options belong to a codec; the new one starts from its defaults.
    setValue("codec.options", {}, { shouldDirty: true });
  }

  return (
    <EditorSection
      title="Codec"
      description="How TestForge reads and writes the agent's messages on that transport."
      disabled={readOnly}
    >
      <Controller
        name="codec.type"
        control={control}
        rules={{ validate: (value) => value !== "" || "Pick a codec." }}
        render={({ fieldState }) => (
          <Field data-invalid={fieldState.invalid}>
            {available === undefined ? (
              <div className="flex gap-2" aria-busy aria-label="Loading codecs">
                <Skeleton className="h-8 w-28 rounded-full" />
                <Skeleton className="h-8 w-32 rounded-full" />
                <Skeleton className="h-8 w-24 rounded-full" />
              </div>
            ) : (
              <div role="radiogroup" aria-label="Codec" className="flex flex-wrap gap-2">
                {available.map((codec) => {
                  const active = codec.codec === selected;
                  return (
                    <button
                      key={codec.codec}
                      type="button"
                      role="radio"
                      aria-checked={active}
                      id={active ? fieldId("codec.type") : undefined}
                      onClick={() => choose(codec.codec)}
                      className={cn(
                        "h-8 rounded-full border px-3 text-sm transition-colors outline-none focus-visible:ring-3 focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50",
                        active
                          ? "border-brand bg-brand/10 font-medium text-foreground"
                          : "text-muted-foreground hover:bg-muted hover:text-foreground",
                      )}
                    >
                      {codec.label}
                    </button>
                  );
                })}
              </div>
            )}
            {fieldState.error ? (
              <FieldError errors={[fieldState.error]} />
            ) : spec ? (
              <FieldDescription>{spec.description}</FieldDescription>
            ) : null}
          </Field>
        )}
      />
      {spec && spec.fields.length > 0 && (
        <div className="grid gap-5 border-t pt-5 sm:grid-cols-2">
          {spec.fields.map((option) => (
            <div key={`${spec.codec}-${option.key}`} className={cn(option.type === "json" && "sm:col-span-2")}>
              <OptionField option={option} readOnly={readOnly} />
            </div>
          ))}
        </div>
      )}
    </EditorSection>
  );
}
