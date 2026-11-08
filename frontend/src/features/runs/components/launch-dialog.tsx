"use client";

import { ArrowRightIcon, CircleAlertIcon, PlayIcon } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { Controller, useForm, useWatch } from "react-hook-form";
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
import { Spinner } from "@/components/ui/spinner";
import { wholeNumberRule } from "@/features/scenarios/components/number-field";
import { SECTIONS, sectionHref } from "@/features/shell/lib/navigation";
import { errorMessage, isApiError } from "@/lib/api/errors";

import { useLaunchRuns } from "../api/runs";
import {
  buildLaunchRequest,
  DEFAULT_CONCURRENCY,
  launchSummary,
  mapLaunchErrors,
  MAX_CONCURRENCY,
  MAX_REPETITIONS,
  type ScenarioLaunchError,
} from "../lib/launch";

type LaunchScenario = { id: string; name: string };

type LaunchDialogProps = {
  slug: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Scenarios to run, in launch order. */
  scenarios: LaunchScenario[];
  onLaunched?: () => void;
};

export function LaunchDialog({ slug, open, onOpenChange, scenarios, onLaunched }: LaunchDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>{scenarios.length === 1 ? `Run “${scenarios[0].name}”` : "Run scenarios"}</DialogTitle>
          <DialogDescription>
            Each run plays the scenario against its agent with a simulated user. Follow them live on the Runs page.
          </DialogDescription>
        </DialogHeader>
        {/* Mounted only while open, so every launch starts fresh. */}
        {open && (
          <LaunchForm
            slug={slug}
            scenarios={scenarios}
            onLaunched={() => {
              onOpenChange(false);
              onLaunched?.();
            }}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}

type LaunchFormValues = { repetitions: string; concurrency: string; label: string };

function LaunchForm({
  slug,
  scenarios,
  onLaunched,
}: {
  slug: string;
  scenarios: LaunchScenario[];
  onLaunched: () => void;
}) {
  const router = useRouter();
  const launch = useLaunchRuns(slug);
  const [scenarioErrors, setScenarioErrors] = useState<ScenarioLaunchError[]>([]);
  const form = useForm<LaunchFormValues>({
    defaultValues: { repetitions: "1", concurrency: String(DEFAULT_CONCURRENCY), label: "" },
  });
  const { control, formState } = form;
  const repetitions = Number(useWatch({ control, name: "repetitions" }));
  const validRepetitions = Number.isInteger(repetitions) && repetitions >= 1 && repetitions <= MAX_REPETITIONS;

  const submit = form.handleSubmit(async (values) => {
    setScenarioErrors([]);
    const ids = scenarios.map((scenario) => scenario.id);
    try {
      const batch = await launch.mutateAsync(
        buildLaunchRequest(ids, {
          repetitions: Number(values.repetitions),
          concurrency: Number(values.concurrency),
          label: values.label,
        }),
      );
      toast.success(batch.total === 1 ? "Run queued" : `${batch.total} runs queued`);
      onLaunched();
      router.push(`${sectionHref(slug, SECTIONS.runs)}?batchId=${encodeURIComponent(batch.id)}`);
    } catch (error) {
      const mapped = mapLaunchErrors(isApiError(error) ? error.fields : {}, ids, scenarios);
      setScenarioErrors(mapped.scenarios);
      for (const [field, message] of Object.entries(mapped.fields)) {
        form.setError(field as keyof LaunchFormValues, { type: "server", message });
      }
      const matched = mapped.scenarios.length + Object.keys(mapped.fields).length;
      if (mapped.other.length > 0 || matched === 0) {
        form.setError("root.server", {
          type: "server",
          message: mapped.other.length > 0 ? mapped.other.join(" ") : errorMessage(error),
        });
      }
    }
  });

  return (
    <form onSubmit={submit} noValidate className="flex flex-col gap-6">
      <FieldGroup>
        <FormError message={formState.errors.root?.server?.message} />
        {scenarioErrors.length > 0 && <ScenarioErrors slug={slug} errors={scenarioErrors} />}
        <div className="grid gap-4 sm:grid-cols-2">
          <Controller
            name="repetitions"
            control={control}
            rules={{ validate: wholeNumberRule(1, MAX_REPETITIONS) }}
            render={({ field, fieldState }) => (
              <Field data-invalid={fieldState.invalid}>
                <FieldLabel htmlFor="launch-repetitions">Repetitions</FieldLabel>
                <Input
                  {...field}
                  id="launch-repetitions"
                  type="number"
                  inputMode="numeric"
                  min={1}
                  max={MAX_REPETITIONS}
                  aria-invalid={fieldState.invalid}
                />
                {fieldState.error ? (
                  <FieldError errors={[fieldState.error]} />
                ) : (
                  <FieldDescription>Runs per scenario, 1 to {MAX_REPETITIONS}.</FieldDescription>
                )}
              </Field>
            )}
          />
          <Controller
            name="concurrency"
            control={control}
            rules={{ validate: wholeNumberRule(1, MAX_CONCURRENCY) }}
            render={({ field, fieldState }) => (
              <Field data-invalid={fieldState.invalid}>
                <FieldLabel htmlFor="launch-concurrency">Concurrency</FieldLabel>
                <Input
                  {...field}
                  id="launch-concurrency"
                  type="number"
                  inputMode="numeric"
                  min={1}
                  max={MAX_CONCURRENCY}
                  aria-invalid={fieldState.invalid}
                />
                {fieldState.error ? (
                  <FieldError errors={[fieldState.error]} />
                ) : (
                  <FieldDescription>Runs of this batch at the same time.</FieldDescription>
                )}
              </Field>
            )}
          />
        </div>
        <Controller
          name="label"
          control={control}
          render={({ field, fieldState }) => (
            <Field data-invalid={fieldState.invalid}>
              <FieldLabel htmlFor="launch-label">Label</FieldLabel>
              <Input
                {...field}
                id="launch-label"
                autoComplete="off"
                maxLength={100}
                placeholder="Nightly regression"
                aria-invalid={fieldState.invalid}
              />
              {fieldState.error ? (
                <FieldError errors={[fieldState.error]} />
              ) : (
                <FieldDescription>Optional. Names the batch on the Runs page.</FieldDescription>
              )}
            </Field>
          )}
        />
        <p className="rounded-lg border bg-muted/30 px-3 py-2 text-sm tabular-nums" aria-live="polite">
          {validRepetitions ? launchSummary(scenarios.length, repetitions) : `${scenarios.length} selected`}
        </p>
      </FieldGroup>
      <DialogFooter>
        <DialogClose asChild>
          <Button type="button" variant="outline">
            Cancel
          </Button>
        </DialogClose>
        <Button type="submit" disabled={formState.isSubmitting || scenarios.length === 0}>
          {formState.isSubmitting ? <Spinner /> : <PlayIcon />}
          Launch
        </Button>
      </DialogFooter>
    </form>
  );
}

function ScenarioErrors({ slug, errors }: { slug: string; errors: ScenarioLaunchError[] }) {
  return (
    <div role="alert" className="flex flex-col gap-2 rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2.5 text-sm">
      <p className="flex items-center gap-2 font-medium">
        <CircleAlertIcon aria-hidden className="size-4 shrink-0 text-destructive" />
        {errors.length === 1 ? "One scenario can't run yet" : `${errors.length} scenarios can't run yet`}
      </p>
      <ul className="flex flex-col gap-1.5">
        {errors.map((error) => (
          <li key={error.scenarioId} className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-0.5">
            <span className="min-w-0">
              <span className="font-medium">{error.name}</span>
              <span className="text-muted-foreground">: {error.message}</span>
            </span>
            <Link
              href={
                error.fix === "persona"
                  ? sectionHref(slug, SECTIONS.personas)
                  : `${sectionHref(slug, SECTIONS.scenarios)}/${error.scenarioId}`
              }
              className="inline-flex shrink-0 items-center gap-1 text-xs font-medium text-brand hover:underline"
            >
              {error.fix === "persona" ? "Fix persona" : "Fix scenario"}
              <ArrowRightIcon aria-hidden className="size-3" />
            </Link>
          </li>
        ))}
      </ul>
    </div>
  );
}
