"use client";

import { ArrowLeftIcon, CopyIcon, SaveIcon, Trash2Icon } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { Controller, FormProvider, useForm, useWatch, type Path } from "react-hook-form";
import { toast } from "sonner";

import { FormError } from "@/components/form-error";
import { PageContainer } from "@/components/page-container";
import { Button } from "@/components/ui/button";
import { Field, FieldError, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { ModalityBadge } from "@/features/agents/components/modality-badge";
import { useBeforeUnload } from "@/features/agents/components/use-before-unload";
import { useConnections } from "@/features/connections/api/connections";
import { usePersonas } from "@/features/personas/api/personas";
import { PersonaDialog } from "@/features/personas/components/persona-dialog";
import { sectionHref, SECTIONS } from "@/features/shell/lib/navigation";
import { useCurrentWorkspace } from "@/features/workspaces/session-context";
import { errorMessage, isApiError } from "@/lib/api/errors";
import type { Agent, Scenario } from "@/lib/api/types";

import {
  useCreateScenario,
  useDuplicateScenario,
  useScenarioGroups,
  useUpdateScenario,
} from "../api/scenarios";
import { criterionSources, simulatorCapability, weightTotal } from "../lib/criteria";
import {
  buildScenarioInput,
  formValuesFromScenario,
  initialScenarioValues,
  mapScenarioFieldErrors,
  type ScenarioFormValues,
} from "../lib/form";
import { canAuthorScenarios } from "../lib/permissions";
import { BasicsSection } from "./basics-section";
import { ConversationSection } from "./conversation-section";
import { CriteriaSection } from "./criteria-section";
import { DeleteScenarioDialog } from "./delete-scenario-dialog";
import { EndConditionsSection, InterruptionsSection } from "./end-conditions-section";
import { fieldId } from "./field-id";
import { ModelsSection } from "./models-section";

/** List-level errors live under `<list>.root` so they do not replace the rows' own errors. */
const LIST_PATHS = new Set(["facts", "criteria", "interruptions"]);
const errorPath = (path: string) => (LIST_PATHS.has(path) ? `${path}.root` : path);

type ScenarioEditorProps = {
  agents: Agent[];
  /** The stored scenario; omitted for a new one. */
  scenario?: Scenario;
  prefill?: { agentId?: string; groupId?: string };
};

export function ScenarioEditor({ agents, scenario, prefill }: ScenarioEditorProps) {
  const router = useRouter();
  const workspace = useCurrentWorkspace();
  const slug = workspace.slug;
  const canAuthor = canAuthorScenarios(workspace.role);
  const readOnly = !canAuthor;
  const personas = usePersonas(slug);
  const groups = useScenarioGroups(slug);
  const connections = useConnections(slug);
  const create = useCreateScenario(slug);
  const update = useUpdateScenario(slug);
  const duplicate = useDuplicateScenario(slug);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [personaDialogOpen, setPersonaDialogOpen] = useState(false);
  const scenariosHref = sectionHref(slug, SECTIONS.scenarios);

  const form = useForm<ScenarioFormValues>({
    defaultValues: scenario
      ? formValuesFromScenario(scenario)
      : initialScenarioValues({
          // Ignore prefilled ids that do not exist (anymore).
          agentId: agents.some((agent) => agent.id === prefill?.agentId) ? prefill?.agentId : undefined,
          groupId: prefill?.groupId,
        }),
  });
  const { control, formState } = form;
  const agentId = useWatch({ control, name: "agentId" });
  const agent = agents.find((item) => item.id === agentId);
  const modality = agent?.modality ?? scenario?.modality;
  const sources = criterionSources(agent ?? (scenario ? { modality: scenario.modality, traceConnectionId: null } : undefined));
  useBeforeUnload(formState.isDirty && !formState.isSubmitting);

  /** Keeps the form valid for a newly picked agent: evidence sources and the simulator connection. */
  function adaptToAgent(next: Agent) {
    const available = criterionSources(next);
    form.getValues("criteria").forEach((criterion, index) => {
      if (!available[criterion.source].available) {
        form.setValue(`criteria.${index}.source`, "transcript", { shouldDirty: true });
      }
    });
    const capability = simulatorCapability(next.modality, form.getValues("simulator.mode"));
    const connection = connections.data?.find((item) => item.id === form.getValues("simulator.connectionId"));
    if (connection && !connection.capabilities.includes(capability)) {
      form.setValue("simulator.connectionId", "", { shouldDirty: true });
    }
  }

  function showErrors(matched: [string, string][], rootMessage: string | null) {
    matched.forEach(([path, message], index) =>
      form.setError(errorPath(path) as Path<ScenarioFormValues>, { type: "server", message }, { shouldFocus: index === 0 }),
    );
    if (rootMessage) form.setError("root.server", { type: "server", message: rootMessage });
    if (matched.length > 0) {
      // Wait a frame: an error can open a collapsed section.
      requestAnimationFrame(() =>
        document.getElementById(fieldId(matched[0][0]))?.scrollIntoView({ block: "center", behavior: "smooth" }),
      );
    }
  }

  const save = form.handleSubmit(async (values) => {
    if (!modality) return;
    const total = weightTotal(values.criteria.map((criterion) => criterion.weight));
    if (total.status !== "ok") {
      showErrors([["criteria", `Weights must total 100 (now ${total.total}). ${total.message}`]], null);
      return;
    }
    try {
      const body = buildScenarioInput(values, modality);
      if (scenario) {
        const updated = await update.mutateAsync({ id: scenario.id, body });
        form.reset(formValuesFromScenario(updated));
        toast.success("Scenario saved");
      } else {
        const created = await create.mutateAsync(body);
        // The page stays mounted (hidden) after navigating; leave it empty for the next scenario.
        form.reset(initialScenarioValues());
        toast.success(`Scenario “${created.name}” created`);
        router.replace(`${scenariosHref}/${created.id}`);
      }
    } catch (error) {
      const { matched, unmatched } = mapScenarioFieldErrors(isApiError(error) ? error.fields : {}, values, modality);
      showErrors(
        matched,
        matched.length === 0 || unmatched.length > 0 ? (unmatched.length > 0 ? unmatched.join(" ") : errorMessage(error)) : null,
      );
    }
  });

  function duplicateScenario() {
    if (!scenario) return;
    duplicate.mutate(scenario.id, {
      onSuccess: (copy) => {
        toast.success(`Duplicated as “${copy.name}”`);
        router.push(`${scenariosHref}/${copy.id}`);
      },
      onError: (error) => toast.error(errorMessage(error)),
    });
  }

  const duplicateButton = (
    <Button
      type="button"
      variant="outline"
      onClick={duplicateScenario}
      disabled={duplicate.isPending || formState.isDirty}
    >
      {duplicate.isPending ? <Spinner /> : <CopyIcon />}
      <span className="max-sm:sr-only">Duplicate</span>
    </Button>
  );

  return (
    <FormProvider {...form}>
      <PageContainer className="gap-6">
        <form
          noValidate
          onSubmit={(event) => {
            // Server errors on whole lists are not tied to an input and would block the next submit.
            form.clearErrors();
            return save(event);
          }}
          className="flex flex-col gap-6"
        >
          <div className="flex flex-col gap-4">
            <Link
              href={scenariosHref}
              className="inline-flex w-fit items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"
            >
              <ArrowLeftIcon className="size-3.5" aria-hidden />
              Scenarios
            </Link>
            <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
              <Controller
                name="name"
                control={control}
                rules={{ validate: (value) => value.trim() !== "" || "Enter a name for the scenario." }}
                render={({ field, fieldState }) => (
                  <Field data-invalid={fieldState.invalid} className="min-w-0 flex-1 gap-1">
                    <FieldLabel htmlFor={fieldId("name")} className="sr-only">
                      Name
                    </FieldLabel>
                    <div className="flex min-w-0 items-center gap-3">
                      <Input
                        {...field}
                        id={fieldId("name")}
                        aria-invalid={fieldState.invalid}
                        placeholder="Untitled scenario"
                        autoComplete="off"
                        maxLength={160}
                        readOnly={readOnly}
                        className="h-10 min-w-0 border-transparent bg-transparent px-2 -ml-2 text-2xl font-semibold tracking-tight shadow-none hover:border-input focus-visible:border-ring md:text-2xl dark:bg-transparent"
                      />
                      {modality && <ModalityBadge modality={modality} />}
                    </div>
                    {fieldState.error && <FieldError errors={[fieldState.error]} />}
                  </Field>
                )}
              />
              {canAuthor && (
                <div className="flex shrink-0 flex-wrap items-center gap-2">
                  {scenario &&
                    (formState.isDirty ? (
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <span tabIndex={0}>{duplicateButton}</span>
                        </TooltipTrigger>
                        <TooltipContent>Save your changes before duplicating.</TooltipContent>
                      </Tooltip>
                    ) : (
                      duplicateButton
                    ))}
                  {scenario && (
                    <Button type="button" variant="outline" onClick={() => setDeleteOpen(true)}>
                      <Trash2Icon />
                      <span className="max-sm:sr-only">Delete</span>
                    </Button>
                  )}
                  <Button
                    type="submit"
                    disabled={formState.isSubmitting || (scenario !== undefined && !formState.isDirty)}
                  >
                    {formState.isSubmitting ? <Spinner /> : <SaveIcon />}
                    {scenario ? "Save" : "Create scenario"}
                  </Button>
                </div>
              )}
            </div>
            {readOnly && (
              <p className="text-sm text-muted-foreground">Viewers can read scenarios but not change them.</p>
            )}
          </div>

          <FormError message={formState.errors.root?.server?.message} />

          <div className="grid items-start gap-6 xl:grid-cols-[minmax(0,1fr)_22rem]">
            <div className="flex min-w-0 flex-col gap-6">
              <BasicsSection
                agents={agents}
                personas={personas.data}
                groups={groups.data}
                agentLocked={scenario !== undefined}
                readOnly={readOnly}
                onAgentChange={adaptToAgent}
                onNewPersona={() => setPersonaDialogOpen(true)}
              />
              <ConversationSection readOnly={readOnly} contextOpen={(scenario?.context ?? "") !== ""} />
              <CriteriaSection sources={sources} readOnly={readOnly} />
              <EndConditionsSection modality={modality} readOnly={readOnly} />
              {modality === "voice" && <InterruptionsSection readOnly={readOnly} />}
            </div>
            <aside className="flex min-w-0 flex-col gap-6 xl:sticky xl:top-20">
              <ModelsSection modality={modality} connections={connections.data} readOnly={readOnly} />
            </aside>
          </div>
        </form>
      </PageContainer>
      {canAuthor && (
        <PersonaDialog
          slug={slug}
          open={personaDialogOpen}
          onOpenChange={setPersonaDialogOpen}
          onSaved={(persona) => form.setValue("personaId", persona.id, { shouldDirty: true, shouldValidate: true })}
        />
      )}
      {scenario && canAuthor && (
        <DeleteScenarioDialog
          slug={slug}
          scenario={scenario}
          open={deleteOpen}
          onOpenChange={setDeleteOpen}
          onDeleted={() => {
            form.reset(form.getValues());
            router.push(scenariosHref);
          }}
        />
      )}
    </FormProvider>
  );
}
