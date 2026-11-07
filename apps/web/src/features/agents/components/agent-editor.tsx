"use client";

import { ArrowLeftIcon, AudioLinesIcon, MessageSquareTextIcon, SaveIcon, Trash2Icon } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { Controller, FormProvider, useForm, useWatch, type Path } from "react-hook-form";
import { toast } from "sonner";

import { FormError } from "@/components/form-error";
import { PageContainer } from "@/components/page-container";
import { Button } from "@/components/ui/button";
import { Field, FieldDescription, FieldError, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import { Textarea } from "@/components/ui/textarea";
import { useConnections } from "@/features/connections/api/connections";
import { sectionHref, SECTIONS } from "@/features/shell/lib/navigation";
import { canManageWorkspace, useCurrentWorkspace } from "@/features/workspaces/session-context";
import { errorMessage, isApiError } from "@/lib/api/errors";
import type { Agent, AgentPreset, Modality } from "@/lib/api/types";
import { cn } from "@/lib/utils";

import { useAgentCodecs, useAgentPresets, useCreateAgent, useUpdateAgent } from "../api/agents";
import { MODALITY_LABELS } from "../lib/display";
import {
  applyPreset,
  buildAgentInput,
  formValuesFromAgent,
  initialFormValues,
  mapServerFieldErrors,
  type AgentFormValues,
} from "../lib/form";
import { CodecSection } from "./codec-section";
import { DeleteAgentDialog } from "./delete-agent-dialog";
import { EditorSection } from "./editor-section";
import { fieldId } from "./field-id";
import { KeyValueTable } from "./key-value-table";
import { ModalityBadge } from "./modality-badge";
import { PresetGrid, PresetMenu } from "./presets";
import { ProbePanel } from "./probe-panel";
import { SecretsSection } from "./secrets-section";
import { TemplateVariables } from "./template-variables";
import { TracesSection } from "./traces-section";
import { TransportSection } from "./transport-section";

const MODALITY_ICONS = { voice: AudioLinesIcon, text: MessageSquareTextIcon } as const;

function ModalityPicker({ value, onChange }: { value: Modality; onChange: (modality: Modality) => void }) {
  return (
    <div role="radiogroup" aria-label="Modality" className="inline-flex rounded-lg border bg-muted/40 p-0.5">
      {(["voice", "text"] as const).map((modality) => {
        const Icon = MODALITY_ICONS[modality];
        const active = modality === value;
        return (
          <button
            key={modality}
            type="button"
            role="radio"
            aria-checked={active}
            onClick={() => onChange(modality)}
            className={cn(
              "inline-flex h-7 items-center gap-1.5 rounded-md px-2.5 text-sm transition-colors outline-none focus-visible:ring-3 focus-visible:ring-ring/50 [&_svg]:size-3.5",
              active ? "bg-background font-medium shadow-xs" : "text-muted-foreground hover:text-foreground",
            )}
          >
            <Icon aria-hidden />
            {MODALITY_LABELS[modality]}
          </button>
        );
      })}
    </div>
  );
}

/** Warns before closing or reloading the tab with unsaved changes. */
function useBeforeUnload(active: boolean) {
  useEffect(() => {
    if (!active) return;
    const handler = (event: BeforeUnloadEvent) => event.preventDefault();
    window.addEventListener("beforeunload", handler);
    return () => window.removeEventListener("beforeunload", handler);
  }, [active]);
}

type AgentEditorProps = {
  /** The stored agent; omitted for a new one. */
  agent?: Agent;
};

export function AgentEditor({ agent }: AgentEditorProps) {
  const router = useRouter();
  const workspace = useCurrentWorkspace();
  const slug = workspace.slug;
  const canManage = canManageWorkspace(workspace.role);
  const readOnly = !canManage;
  const codecs = useAgentCodecs();
  const presets = useAgentPresets();
  const connections = useConnections(slug);
  const create = useCreateAgent(slug);
  const update = useUpdateAgent(slug);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const agentsHref = sectionHref(slug, SECTIONS.agents);

  const form = useForm<AgentFormValues>({
    // The edit page waits for the codec catalog, so it is available here.
    defaultValues: agent ? formValuesFromAgent(agent, codecs.data) : initialFormValues(),
  });
  const { control, formState } = form;
  const modality = useWatch({ control, name: "modality" });
  useBeforeUnload(formState.isDirty && !formState.isSubmitting);

  function changeModality(next: Modality) {
    if (next === modality) return;
    form.setValue("modality", next, { shouldDirty: true });
    // Codecs belong to one modality.
    form.setValue("codec.type", "", { shouldDirty: true });
    form.setValue("codec.options", {}, { shouldDirty: true });
  }

  function applyPresetToForm(preset: AgentPreset) {
    // keepDefaultValues: the form stays dirty against the stored (or empty) definition.
    form.reset(applyPreset(form.getValues(), preset), { keepDefaultValues: true });
    toast.success(`Applied “${preset.label}”`, { description: "Review the values, fill in the secrets and save." });
  }

  const save = form.handleSubmit(async (values) => {
    try {
      const body = buildAgentInput(values);
      if (agent) {
        const updated = await update.mutateAsync({ id: agent.id, body });
        form.reset(formValuesFromAgent(updated, codecs.data));
        toast.success("Agent saved");
      } else {
        const created = await create.mutateAsync(body);
        // The page stays mounted (hidden) after navigating; leave it empty for the next agent.
        form.reset(initialFormValues());
        toast.success(`Agent “${created.name}” created`);
        router.replace(`${agentsHref}/${created.id}`);
      }
    } catch (error) {
      const { matched, unmatched } = mapServerFieldErrors(
        isApiError(error) ? error.fields : {},
        values,
        codecs.data ?? [],
      );
      matched.forEach(([path, message], index) =>
        form.setError(path as Path<AgentFormValues>, { type: "server", message }, { shouldFocus: index === 0 }),
      );
      if (matched.length === 0 || unmatched.length > 0) {
        form.setError("root.server", {
          type: "server",
          message: unmatched.length > 0 ? unmatched.join(" ") : errorMessage(error),
        });
      }
      if (matched.length > 0) {
        document.getElementById(fieldId(matched[0][0]))?.scrollIntoView({ block: "center", behavior: "smooth" });
      }
    }
  });

  return (
    <FormProvider {...form}>
      <PageContainer className="gap-6">
        <form
          noValidate
          onSubmit={(event) => {
            // Server errors on whole lists (e.g. `secrets`) are not tied to an input and would
            // otherwise block the next submit.
            form.clearErrors();
            return save(event);
          }}
          className="flex flex-col gap-6"
        >
          <div className="flex flex-col gap-4">
            <Link
              href={agentsHref}
              className="inline-flex w-fit items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"
            >
              <ArrowLeftIcon className="size-3.5" aria-hidden />
              Agents
            </Link>
            <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
              <Controller
                name="name"
                control={control}
                rules={{ validate: (value) => value.trim() !== "" || "Enter a name for the agent." }}
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
                        placeholder="Untitled agent"
                        autoComplete="off"
                        maxLength={100}
                        readOnly={readOnly}
                        className="h-10 min-w-0 border-transparent bg-transparent px-2 -ml-2 text-2xl font-semibold tracking-tight shadow-none hover:border-input focus-visible:border-ring md:text-2xl dark:bg-transparent"
                      />
                      {agent && <ModalityBadge modality={agent.modality} />}
                    </div>
                    {fieldState.error && <FieldError errors={[fieldState.error]} />}
                  </Field>
                )}
              />
              <div className="flex shrink-0 flex-wrap items-center gap-2">
                {!agent && canManage && <ModalityPicker value={modality} onChange={changeModality} />}
                {agent && canManage && (
                  <PresetMenu presets={presets.data} modality={agent.modality} onApply={applyPresetToForm} />
                )}
                {agent && canManage && (
                  <Button type="button" variant="outline" onClick={() => setDeleteOpen(true)}>
                    <Trash2Icon />
                    <span className="max-sm:sr-only">Delete</span>
                  </Button>
                )}
                {canManage && (
                  <Button type="submit" disabled={formState.isSubmitting || (agent !== undefined && !formState.isDirty)}>
                    {formState.isSubmitting ? <Spinner /> : <SaveIcon />}
                    {agent ? "Save" : "Create agent"}
                  </Button>
                )}
              </div>
            </div>
            {readOnly && (
              <p className="text-sm text-muted-foreground">
                Only owners and admins can change agents. You can still inspect the definition
                {workspace.role !== "viewer" && " and run a probe"}.
              </p>
            )}
          </div>

          <FormError message={formState.errors.root?.server?.message} />

          {!agent && canManage && <PresetGrid presets={presets.data} onApply={applyPresetToForm} />}

          <div className="grid items-start gap-6 lg:grid-cols-[minmax(0,1fr)_22rem]">
            <div className="flex min-w-0 flex-col gap-6">
              <EditorSection
                title="Description"
                description="Context about the agent that every scenario shares: who it serves, what it can do, its rules."
                disabled={readOnly}
              >
                <Controller
                  name="description"
                  control={control}
                  render={({ field, fieldState }) => (
                    <Field data-invalid={fieldState.invalid}>
                      <FieldLabel htmlFor={fieldId("description")} className="sr-only">
                        Description
                      </FieldLabel>
                      <Textarea
                        {...field}
                        id={fieldId("description")}
                        aria-invalid={fieldState.invalid}
                        maxLength={20000}
                        placeholder="A billing support agent for Acme. It can look up invoices and issue refunds up to $50."
                        className="min-h-28"
                      />
                      {fieldState.error ? (
                        <FieldError errors={[fieldState.error]} />
                      ) : (
                        <FieldDescription>Simulated users and judges read this before each conversation.</FieldDescription>
                      )}
                    </Field>
                  )}
                />
              </EditorSection>
              <TransportSection readOnly={readOnly} />
              <CodecSection codecs={codecs.data} readOnly={readOnly} />
              <EditorSection
                title="Variables"
                description={
                  <>
                    Non-secret values referenced as <code className="font-mono">{"{{var.<key>}}"}</code>.
                  </>
                }
                disabled={readOnly}
              >
                <KeyValueTable name="variables" label="Variables" addLabel="Add variable" readOnly={readOnly} />
              </EditorSection>
              <SecretsSection stored={agent} readOnly={readOnly} />
            </div>
            <aside className="flex min-w-0 flex-col gap-6 *:shrink-0 lg:sticky lg:top-20 lg:-m-1 lg:max-h-[calc(100svh-6rem)] lg:overflow-y-auto lg:p-1">
              <ProbePanel
                slug={slug}
                agentId={agent?.id}
                modality={modality}
                canProbe={workspace.role !== "viewer"}
                dirty={formState.isDirty}
              />
              <TracesSection connections={connections.data} readOnly={readOnly} />
              <TemplateVariables />
            </aside>
          </div>
        </form>
      </PageContainer>
      {agent && canManage && (
        <DeleteAgentDialog
          slug={slug}
          agent={agent}
          open={deleteOpen}
          onOpenChange={setDeleteOpen}
          onDeleted={() => {
            form.reset(form.getValues());
            router.push(agentsHref);
          }}
        />
      )}
    </FormProvider>
  );
}
