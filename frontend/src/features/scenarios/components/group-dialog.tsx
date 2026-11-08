"use client";

import { Controller, useForm } from "react-hook-form";
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
import { Field, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import { Textarea } from "@/components/ui/textarea";
import { applyApiError } from "@/lib/forms";
import type { ScenarioGroup } from "@/lib/api/types";

import { useCreateScenarioGroup, useUpdateScenarioGroup } from "../api/scenarios";

type GroupDialogProps = {
  slug: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** The group to rename; omit to create one. */
  group?: ScenarioGroup | null;
  onSaved?: (group: ScenarioGroup) => void;
};

type GroupFormValues = { name: string; description: string };

export function GroupDialog({ slug, open, onOpenChange, group, onSaved }: GroupDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{group ? "Rename group" : "New group"}</DialogTitle>
          <DialogDescription>Groups keep related scenarios together, e.g. a suite you run before each release.</DialogDescription>
        </DialogHeader>
        {open && (
          <GroupForm
            slug={slug}
            group={group ?? undefined}
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

function GroupForm({ slug, group, onSaved }: { slug: string; group?: ScenarioGroup; onSaved: (group: ScenarioGroup) => void }) {
  const create = useCreateScenarioGroup(slug);
  const update = useUpdateScenarioGroup(slug);
  const form = useForm<GroupFormValues>({
    defaultValues: { name: group?.name ?? "", description: group?.description ?? "" },
  });

  const submit = form.handleSubmit(async (values) => {
    const body = { name: values.name.trim(), description: values.description.trim() };
    try {
      const saved = group ? await update.mutateAsync({ id: group.id, body }) : await create.mutateAsync(body);
      toast.success(group ? `Group “${saved.name}” saved` : `Group “${saved.name}” created`);
      onSaved(saved);
    } catch (error) {
      applyApiError(error, form.setError, ["name", "description"]);
    }
  });

  return (
    <form onSubmit={submit} noValidate className="flex flex-col gap-6">
      <FieldGroup>
        <FormError message={form.formState.errors.root?.server?.message} />
        <Controller
          name="name"
          control={form.control}
          rules={{ validate: (value) => value.trim() !== "" || "Enter a name for the group." }}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor="group-name">Name</FieldLabel>
              <Input
                {...field}
                id="group-name"
                autoComplete="off"
                maxLength={120}
                placeholder="Billing regression"
                aria-invalid={fieldState.invalid}
                autoFocus
              />
              {fieldState.error && <FieldError errors={[fieldState.error]} />}
            </Field>
          )}
        />
        <Controller
          name="description"
          control={form.control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor="group-description">Description</FieldLabel>
              <Textarea {...field} id="group-description" maxLength={2000} placeholder="Optional" aria-invalid={fieldState.invalid} />
              {fieldState.error && <FieldError errors={[fieldState.error]} />}
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
          {group ? "Save" : "Create group"}
        </Button>
      </DialogFooter>
    </form>
  );
}
