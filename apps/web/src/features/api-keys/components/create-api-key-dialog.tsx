"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { TriangleAlertIcon } from "lucide-react";
import { useState } from "react";
import { Controller, useForm } from "react-hook-form";
import { z } from "zod";

import { FormError } from "@/components/form-error";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
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
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import type { CreatedApiKey } from "@/lib/api/types";
import { applyApiError } from "@/lib/forms";

import { useCreateApiKey } from "../api/api-keys";
import { CopyButton } from "./copy-button";

// Mirrors `CreateApiKeyRequest` in backend/api/openapi.yaml.
const schema = z.object({
  name: z
    .string()
    .trim()
    .min(1, "Enter a name for the key.")
    .max(100, "Use 100 characters or fewer."),
  expiresInDays: z
    .string()
    .trim()
    .refine(
      (value) =>
        value === "" ||
        (/^\d+$/.test(value) && Number(value) >= 1 && Number(value) <= 3650),
      "Enter a whole number of days between 1 and 3650, or leave it empty.",
    ),
});

type Values = z.infer<typeof schema>;

type CreateApiKeyDialogProps = {
  slug: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
};

export function CreateApiKeyDialog({ slug, open, onOpenChange }: CreateApiKeyDialogProps) {
  const createKey = useCreateApiKey(slug);
  const [created, setCreated] = useState<CreatedApiKey | null>(null);
  const form = useForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { name: "", expiresInDays: "" },
  });

  // Forget the secret once the dialog has closed (after its exit animation): it is shown only once.
  function resetDialog() {
    setCreated(null);
    form.reset();
  }

  const onSubmit = form.handleSubmit((values) =>
    createKey
      .mutateAsync({
        name: values.name,
        expiresInDays: values.expiresInDays === "" ? null : Number(values.expiresInDays),
      })
      .then(setCreated, (error) =>
        applyApiError(error, form.setError, ["name", "expiresInDays"]),
      ),
  );

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className="sm:max-w-lg"
        onInteractOutside={(event) => created && event.preventDefault()}
        onCloseAutoFocus={resetDialog}
      >
        {created ? (
          <div className="flex flex-col gap-6">
            <DialogHeader>
              <DialogTitle>Save your API key</DialogTitle>
              <DialogDescription>
                Use it as a bearer token: <code className="font-mono text-xs">Authorization: Bearer tf_…</code>
              </DialogDescription>
            </DialogHeader>
            <Alert>
              <TriangleAlertIcon className="text-warning!" />
              <AlertTitle>This is the only time you will see this key</AlertTitle>
              <AlertDescription>
                Copy it now and store it somewhere safe. If you lose it, revoke it and create a new one.
              </AlertDescription>
            </Alert>
            <Field>
              <FieldLabel htmlFor="created-api-key">{created.name}</FieldLabel>
              <div className="flex gap-2">
                <Input
                  id="created-api-key"
                  readOnly
                  value={created.secret}
                  className="font-mono text-xs"
                  onFocus={(event) => event.currentTarget.select()}
                />
                <CopyButton value={created.secret} />
              </div>
            </Field>
            <DialogFooter>
              <Button type="button" onClick={() => onOpenChange(false)}>
                I have saved my key
              </Button>
            </DialogFooter>
          </div>
        ) : (
          <form onSubmit={onSubmit} noValidate className="flex flex-col gap-6">
            <DialogHeader>
              <DialogTitle>Create API key</DialogTitle>
              <DialogDescription>
                API keys give scripts, CI jobs and the CLI access to this workspace only.
              </DialogDescription>
            </DialogHeader>
            <FieldGroup>
              <FormError message={form.formState.errors.root?.server?.message} />
              <Controller
                name="name"
                control={form.control}
                render={({ field, fieldState }) => (
                  <Field data-invalid={fieldState.invalid}>
                    <FieldLabel htmlFor="api-key-name">Name</FieldLabel>
                    <Input
                      {...field}
                      id="api-key-name"
                      placeholder="GitHub Actions"
                      autoComplete="off"
                      aria-invalid={fieldState.invalid}
                      autoFocus
                    />
                    {fieldState.invalid ? (
                      <FieldError errors={[fieldState.error]} />
                    ) : (
                      <FieldDescription>Helps you recognise where the key is used.</FieldDescription>
                    )}
                  </Field>
                )}
              />
              <Controller
                name="expiresInDays"
                control={form.control}
                render={({ field, fieldState }) => (
                  <Field data-invalid={fieldState.invalid}>
                    <FieldLabel htmlFor="api-key-expiry">Expires after (days)</FieldLabel>
                    <Input
                      {...field}
                      id="api-key-expiry"
                      type="number"
                      inputMode="numeric"
                      min={1}
                      max={3650}
                      placeholder="Never"
                      aria-invalid={fieldState.invalid}
                    />
                    {fieldState.invalid ? (
                      <FieldError errors={[fieldState.error]} />
                    ) : (
                      <FieldDescription>Optional. Leave empty for a key that does not expire.</FieldDescription>
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
              <Button type="submit" disabled={form.formState.isSubmitting}>
                {form.formState.isSubmitting && <Spinner />}
                Create key
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
}
