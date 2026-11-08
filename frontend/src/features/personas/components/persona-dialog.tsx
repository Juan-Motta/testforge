"use client";

import { Controller, useForm, useWatch, type Path } from "react-hook-form";
import { toast } from "sonner";

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
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Spinner } from "@/components/ui/spinner";
import { Textarea } from "@/components/ui/textarea";
import { useConnections } from "@/features/connections/api/connections";
import { errorMessage, isApiError } from "@/lib/api/errors";
import type { Persona } from "@/lib/api/types";

import { useCreatePersona, useUpdatePersona } from "../api/personas";
import {
  buildPersonaInput,
  initialPersonaValues,
  LOCALE_SUGGESTIONS,
  mapPersonaFieldErrors,
  personaFormValues,
  TEMPERAMENTS,
  TEMPERAMENT_LABELS,
  voiceConnections,
  type PersonaFormValues,
} from "../lib/form";

// Radix Select does not accept an empty value, so "no voice" has its own token.
const NONE = "none";

type PersonaDialogProps = {
  slug: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** The persona to edit; omit to create one. */
  persona?: Persona | null;
  onSaved?: (persona: Persona) => void;
};

export function PersonaDialog({ slug, open, onOpenChange, persona, onSaved }: PersonaDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{persona ? `Edit “${persona.name}”` : "New persona"}</DialogTitle>
          <DialogDescription>
            A simulated user your scenarios can reuse: how they talk, in which language and with which voice.
          </DialogDescription>
        </DialogHeader>
        {/* Mounted only while open, so every visit starts from the stored values. */}
        {open && (
          <PersonaForm
            slug={slug}
            persona={persona ?? undefined}
            onSaved={(saved) => {
              onOpenChange(false);
              onSaved?.(saved);
            }}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

function PersonaForm({ slug, persona, onSaved }: { slug: string; persona?: Persona; onSaved: (persona: Persona) => void }) {
  const create = useCreatePersona(slug);
  const update = useUpdatePersona(slug);
  const connections = useConnections(slug);
  const form = useForm<PersonaFormValues>({
    defaultValues: persona ? personaFormValues(persona) : initialPersonaValues(),
  });
  const { control, formState } = form;
  const voiceConnectionId = useWatch({ control, name: "voice.connectionId" });
  const options = voiceConnections(connections.data ?? []);

  const submit = form.handleSubmit(async (values) => {
    try {
      const body = buildPersonaInput(values);
      const saved = persona ? await update.mutateAsync({ id: persona.id, body }) : await create.mutateAsync(body);
      toast.success(persona ? `Persona “${saved.name}” saved` : `Persona “${saved.name}” created`);
      onSaved(saved);
    } catch (error) {
      const { matched, unmatched } = mapPersonaFieldErrors(isApiError(error) ? error.fields : {});
      matched.forEach(([path, message], index) =>
        form.setError(path as Path<PersonaFormValues>, { type: "server", message }, { shouldFocus: index === 0 }),
      );
      if (matched.length === 0 || unmatched.length > 0) {
        form.setError("root.server", {
          type: "server",
          message: unmatched.length > 0 ? unmatched.join(" ") : errorMessage(error),
        });
      }
    }
  });

  return (
    <form onSubmit={submit} noValidate className="flex flex-col gap-6">
      <FieldGroup>
        <FormError message={formState.errors.root?.server?.message} />
        <Controller
          name="name"
          control={control}
          rules={{ validate: (value) => value.trim() !== "" || "Enter a name for the persona." }}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor="persona-name">Name</FieldLabel>
              <Input
                {...field}
                id="persona-name"
                autoComplete="off"
                maxLength={120}
                placeholder="Hurried commuter"
                aria-invalid={fieldState.invalid}
                autoFocus
              />
              {fieldState.error && <FieldError errors={[fieldState.error]} />}
            </Field>
          )}
        />
        <div className="grid gap-4 sm:grid-cols-2">
          <Controller
            name="locale"
            control={control}
            rules={{ validate: (value) => value.trim() !== "" || "Enter a language tag, e.g. en-US." }}
            render={({ field, fieldState }) => (
              <Field data-invalid={fieldState.invalid}>
                <FieldLabel htmlFor="persona-locale">Locale</FieldLabel>
                <Input
                  {...field}
                  id="persona-locale"
                  autoComplete="off"
                  spellCheck={false}
                  placeholder="en-US"
                  aria-invalid={fieldState.invalid}
                />
                {fieldState.error ? (
                  <FieldError errors={[fieldState.error]} />
                ) : (
                  <div className="flex flex-wrap gap-1" aria-label="Suggested locales">
                    {LOCALE_SUGGESTIONS.map((locale) => (
                      <Button
                        key={locale}
                        type="button"
                        variant={field.value === locale ? "secondary" : "ghost"}
                        size="xs"
                        className="font-mono"
                        onClick={() => field.onChange(locale)}
                      >
                        {locale}
                      </Button>
                    ))}
                  </div>
                )}
              </Field>
            )}
          />
          <Controller
            name="temperament"
            control={control}
            render={({ field, fieldState }) => (
              <Field data-invalid={fieldState.invalid}>
                <FieldLabel htmlFor="persona-temperament">Temperament</FieldLabel>
                <Select value={field.value} onValueChange={field.onChange}>
                  <SelectTrigger id="persona-temperament" className="w-full" aria-invalid={fieldState.invalid}>
                    <SelectValue>{TEMPERAMENT_LABELS[field.value]}</SelectValue>
                  </SelectTrigger>
                  <SelectContent>
                    {TEMPERAMENTS.map((temperament) => (
                      <SelectItem key={temperament.value} value={temperament.value}>
                        <span className="flex flex-col">
                          <span>{temperament.label}</span>
                          <span className="text-xs text-muted-foreground">{temperament.description}</span>
                        </span>
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                {fieldState.error ? (
                  <FieldError errors={[fieldState.error]} />
                ) : (
                  <FieldDescription>
                    {TEMPERAMENTS.find((temperament) => temperament.value === field.value)?.description}
                  </FieldDescription>
                )}
              </Field>
            )}
          />
        </div>
        <Controller
          name="notes"
          control={control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor="persona-notes">Notes</FieldLabel>
              <Textarea
                {...field}
                id="persona-notes"
                maxLength={2000}
                placeholder="Calls from a noisy car, speaks fast and interrupts when the agent is slow."
                className="min-h-24"
                aria-invalid={fieldState.invalid}
              />
              {fieldState.error ? (
                <FieldError errors={[fieldState.error]} />
              ) : (
                <FieldDescription>Background and habits the simulated user plays.</FieldDescription>
              )}
            </Field>
          )}
        />
        <div className="grid gap-4 sm:grid-cols-2">
          <Controller
            name="voice.connectionId"
            control={control}
            render={({ field, fieldState }) => (
              <Field data-invalid={fieldState.invalid}>
                <FieldLabel htmlFor="persona-voice-connection">Voice connection</FieldLabel>
                <Select
                  value={field.value === "" ? NONE : field.value}
                  onValueChange={(value) => field.onChange(value === NONE ? "" : value)}
                >
                  <SelectTrigger id="persona-voice-connection" className="w-full" aria-invalid={fieldState.invalid}>
                    <SelectValue placeholder="No voice" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value={NONE}>No voice</SelectItem>
                    {options.map((connection) => (
                      <SelectItem key={connection.id} value={connection.id}>
                        {connection.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                {fieldState.error ? (
                  <FieldError errors={[fieldState.error]} />
                ) : connections.data && options.length === 0 ? (
                  <FieldDescription>Add a text-to-speech or realtime connection to give personas a voice.</FieldDescription>
                ) : (
                  <FieldDescription>Optional. Used by voice scenarios.</FieldDescription>
                )}
              </Field>
            )}
          />
          <Controller
            name="voice.voice"
            control={control}
            rules={{
              validate: (value, values) =>
                values.voice.connectionId === "" || value.trim() !== "" || "Enter the provider's voice name.",
            }}
            render={({ field, fieldState }) => (
              <Field data-invalid={fieldState.invalid}>
                <FieldLabel htmlFor="persona-voice-name">Voice</FieldLabel>
                <Input
                  {...field}
                  id="persona-voice-name"
                  autoComplete="off"
                  spellCheck={false}
                  maxLength={100}
                  placeholder="alloy"
                  disabled={voiceConnectionId === ""}
                  aria-invalid={fieldState.invalid}
                />
                {fieldState.error ? (
                  <FieldError errors={[fieldState.error]} />
                ) : (
                  <FieldDescription>Provider voice name, e.g. es-MX-DaliaNeural.</FieldDescription>
                )}
              </Field>
            )}
          />
        </div>
        <Controller
          name="voice.model"
          control={control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor="persona-voice-model">Model</FieldLabel>
              <Input
                {...field}
                id="persona-voice-model"
                autoComplete="off"
                spellCheck={false}
                maxLength={100}
                placeholder="gpt-4o-mini-tts"
                disabled={voiceConnectionId === ""}
                aria-invalid={fieldState.invalid}
              />
              {fieldState.error ? (
                <FieldError errors={[fieldState.error]} />
              ) : (
                <FieldDescription>
                  Optional. OpenAI default gpt-4o-mini-tts; Azure OpenAI deployment name.
                </FieldDescription>
              )}
            </Field>
          )}
        />
      </FieldGroup>
      <DialogFooter>
        <DialogClose asChild>
          <Button type="button" variant="outline">
            Cancel
          </Button>
        </DialogClose>
        <Button type="submit" disabled={formState.isSubmitting}>
          {formState.isSubmitting && <Spinner />}
          {persona ? "Save changes" : "Create persona"}
        </Button>
      </DialogFooter>
    </form>
  );
}
