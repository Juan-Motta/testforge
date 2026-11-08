"use client";

import { ArrowLeftIcon, SaveIcon, SparklesIcon, Trash2Icon } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { Controller, FormProvider, useForm, type Path } from "react-hook-form";
import { toast } from "sonner";

import { FormError } from "@/components/form-error";
import { PageContainer } from "@/components/page-container";
import { Button } from "@/components/ui/button";
import { Field, FieldError, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import { useBeforeUnload } from "@/features/agents/components/use-before-unload";
import { useConnections } from "@/features/connections/api/connections";
import { sectionHref, SECTIONS } from "@/features/shell/lib/navigation";
import { useCurrentWorkspace } from "@/features/workspaces/session-context";
import { errorMessage, isApiError } from "@/lib/api/errors";
import type { SandboxAgent, SandboxDraft } from "@/lib/api/types";

import { useCreateSandboxAgent, useUpdateSandboxAgent } from "../api/sandbox";
import {
  applyAgentDraft,
  buildSandboxInput,
  formValuesFromSandbox,
  generateContext,
  initialSandboxValues,
  mapSandboxFieldErrors,
  MAX_TOOLS,
  toolFormValues,
  type SandboxFormValues,
} from "../lib/form";
import { canEditSandbox, canLinkSandbox } from "../lib/permissions";
import { DeleteSandboxDialog } from "./delete-sandbox-dialog";
import { fieldId } from "./field-id";
import { GenerateDialog } from "./generate-dialog";
import { InstructionsSection } from "./instructions-section";
import { LinkSandboxButton } from "./link-sandbox-button";
import { MemorySection } from "./memory-section";
import { ModelSection } from "./model-section";
import { SandboxSidePanels } from "./sandbox-side-panels";
import { ToolsSection } from "./tools-section";

const FORM_ID = "sandbox-agent-form";

type SandboxEditorProps = {
  /** The stored agent; omitted for a new one. */
  agent?: SandboxAgent;
};

export function SandboxEditor({ agent }: SandboxEditorProps) {
  const router = useRouter();
  const workspace = useCurrentWorkspace();
  const slug = workspace.slug;
  const canEdit = canEditSandbox(workspace.role);
  const readOnly = !canEdit;
  const connections = useConnections(slug);
  const create = useCreateSandboxAgent(slug);
  const update = useUpdateSandboxAgent(slug);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [generating, setGenerating] = useState<"agent" | "tool" | null>(null);
  const sandboxHref = sectionHref(slug, SECTIONS.sandbox);

  const form = useForm<SandboxFormValues>({
    defaultValues: agent ? formValuesFromSandbox(agent) : initialSandboxValues(),
  });
  const { control, formState } = form;
  useBeforeUnload(formState.isDirty && !formState.isSubmitting);

  function applyDraft(draft: SandboxDraft) {
    if (draft.agent) {
      // keepDefaultValues: the form stays dirty against the stored (or empty) definition.
      form.reset(applyAgentDraft(form.getValues(), draft.agent), { keepDefaultValues: true });
      toast.success("Draft applied", { description: "Review the agent and save it." });
    }
    if (draft.tool) {
      if (form.getValues("tools").length >= MAX_TOOLS) {
        toast.error(`An agent can have at most ${MAX_TOOLS} tools.`);
        return;
      }
      form.setValue("tools", [...form.getValues("tools"), toolFormValues(draft.tool)], { shouldDirty: true });
      toast.success(`Tool “${draft.tool.name}” added`, { description: "Review it and save the agent." });
    }
  }

  const save = form.handleSubmit(async (values) => {
    try {
      const body = buildSandboxInput(values);
      if (agent) {
        const updated = await update.mutateAsync({ id: agent.id, body });
        form.reset(formValuesFromSandbox(updated));
        toast.success("Sandbox agent saved");
      } else {
        const created = await create.mutateAsync(body);
        // The page stays mounted (hidden) after navigating; leave it empty for the next agent.
        form.reset(initialSandboxValues());
        toast.success(`Sandbox agent “${created.name}” created`);
        router.replace(`${sandboxHref}/${created.id}`);
      }
    } catch (error) {
      const { matched, unmatched } = mapSandboxFieldErrors(isApiError(error) ? error.fields : {}, values);
      matched.forEach(([path, message], index) =>
        form.setError(path as Path<SandboxFormValues>, { type: "server", message }, { shouldFocus: index === 0 }),
      );
      if (matched.length === 0 || unmatched.length > 0) {
        form.setError("root.server", {
          type: "server",
          message: unmatched.length > 0 ? unmatched.join(" ") : errorMessage(error),
        });
      }
      if (matched.length > 0) {
        const target = matched[0][0].replace(/\.root$/, "");
        document.getElementById(fieldId(target))?.scrollIntoView({ block: "center", behavior: "smooth" });
      }
    }
  });

  return (
    <FormProvider {...form}>
      <PageContainer className="gap-6">
        <div className="flex flex-col gap-4">
          <Link
            href={sandboxHref}
            className="inline-flex w-fit items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"
          >
            <ArrowLeftIcon className="size-3.5" aria-hidden />
            Sandbox
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
                  <Input
                    {...field}
                    id={fieldId("name")}
                    form={FORM_ID}
                    aria-invalid={fieldState.invalid}
                    placeholder="Untitled sandbox agent"
                    autoComplete="off"
                    maxLength={100}
                    readOnly={readOnly}
                    className="-ml-2 h-10 min-w-0 border-transparent bg-transparent px-2 text-2xl font-semibold tracking-tight shadow-none hover:border-input focus-visible:border-ring md:text-2xl dark:bg-transparent"
                  />
                  {fieldState.error && <FieldError errors={[fieldState.error]} />}
                </Field>
              )}
            />
            <div className="flex shrink-0 flex-wrap items-center gap-2">
              {canEdit && (
                <Button type="button" variant="outline" onClick={() => setGenerating("agent")}>
                  <SparklesIcon />
                  <span className="max-sm:sr-only">Generate with AI</span>
                </Button>
              )}
              {agent && <LinkSandboxButton slug={slug} agentId={agent.id} canLink={canLinkSandbox(workspace.role)} />}
              {agent && canEdit && (
                <Button type="button" variant="outline" onClick={() => setDeleteOpen(true)}>
                  <Trash2Icon />
                  <span className="max-sm:sr-only">Delete</span>
                </Button>
              )}
              {canEdit && (
                <Button
                  type="submit"
                  form={FORM_ID}
                  disabled={formState.isSubmitting || (agent !== undefined && !formState.isDirty)}
                >
                  {formState.isSubmitting ? <Spinner /> : <SaveIcon />}
                  {agent ? "Save" : "Create agent"}
                </Button>
              )}
            </div>
          </div>
          {readOnly && (
            <p className="text-sm text-muted-foreground">
              Viewers can inspect sandbox agents and chat with them but not change them.
            </p>
          )}
        </div>

        <FormError message={formState.errors.root?.server?.message} />

        <div className="grid items-start gap-6 lg:grid-cols-[minmax(0,1fr)_24rem]">
          <form
            id={FORM_ID}
            noValidate
            onSubmit={(event) => {
              // Server errors on the whole tool list are not tied to an input and would block the next submit.
              form.clearErrors();
              return save(event);
            }}
            className="flex min-w-0 flex-col gap-6"
          >
            <InstructionsSection readOnly={readOnly} />
            <ModelSection connections={connections.data} connectionsHref={sectionHref(slug, SECTIONS.connections)} readOnly={readOnly} />
            <MemorySection readOnly={readOnly} />
            <ToolsSection readOnly={readOnly} onGenerate={canEdit ? () => setGenerating("tool") : undefined} />
          </form>
          <aside className="flex min-w-0 flex-col gap-6 *:shrink-0 lg:sticky lg:top-20 lg:-m-1 lg:max-h-[calc(100svh-6rem)] lg:overflow-y-auto lg:p-1">
            {agent && !canEdit ? (
              <p className="rounded-xl border border-dashed p-4 text-sm text-muted-foreground">
                Viewers can&apos;t chat with sandbox agents.
              </p>
            ) : agent ? (
              <SandboxSidePanels
                slug={slug}
                agentId={agent.id}
                longTermMemory={agent.memory.longTerm}
                canDeleteMemories={canEdit}
                dirty={formState.isDirty}
              />
            ) : (
              <p className="rounded-xl border border-dashed p-4 text-sm text-muted-foreground">
                Create the agent to chat with it in the playground.
              </p>
            )}
          </aside>
        </div>
      </PageContainer>
      {canEdit && (
        <GenerateDialog
          slug={slug}
          kind={generating ?? "agent"}
          open={generating !== null}
          onOpenChange={(open) => !open && setGenerating(null)}
          connections={connections.data}
          defaults={{ connectionId: form.getValues("connectionId"), model: form.getValues("model") }}
          context={generating === "tool" ? () => generateContext(form.getValues()) : undefined}
          onDraft={applyDraft}
        />
      )}
      {agent && canEdit && (
        <DeleteSandboxDialog
          slug={slug}
          agent={agent}
          open={deleteOpen}
          onOpenChange={setDeleteOpen}
          onDeleted={() => {
            form.reset(form.getValues());
            router.push(sandboxHref);
          }}
        />
      )}
    </FormProvider>
  );
}
