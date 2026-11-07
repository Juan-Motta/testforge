"use client";

import { BotIcon, ClipboardListIcon, PlusIcon, RotateCwIcon, SearchXIcon } from "lucide-react";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useCallback, useMemo, useState } from "react";

import { EmptyState } from "@/components/empty-state";
import { PageContainer } from "@/components/page-container";
import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { useAgents } from "@/features/agents/api/agents";
import { usePersonas } from "@/features/personas/api/personas";
import { SECTIONS, sectionHref } from "@/features/shell/lib/navigation";
import { useCurrentWorkspace } from "@/features/workspaces/session-context";
import { errorMessage } from "@/lib/api/errors";
import type { ScenarioGroup } from "@/lib/api/types";

import { useScenarioGroups, useScenarios } from "../api/scenarios";
import { filtersFromSearchParams, filtersToQueryString, NO_FILTERS, UNGROUPED, type ScenarioFilters } from "../lib/filters";
import { canAuthorScenarios } from "../lib/permissions";
import { DeleteGroupDialog } from "./delete-group-dialog";
import { GroupDialog } from "./group-dialog";
import { GroupSidebar } from "./group-sidebar";
import { ScenarioList } from "./scenario-list";
import { ScenarioToolbar } from "./scenario-toolbar";

export function ScenariosPage() {
  const workspace = useCurrentWorkspace();
  const slug = workspace.slug;
  const canAuthor = canAuthorScenarios(workspace.role);
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const queryString = searchParams.toString();
  const filters = useMemo(() => filtersFromSearchParams(new URLSearchParams(queryString)), [queryString]);

  const agents = useAgents(slug);
  const personas = usePersonas(slug);
  const groups = useScenarioGroups(slug);
  // The unfiltered list gives the sidebar counts and the tag options.
  const all = useScenarios(slug, NO_FILTERS);
  const scenarios = useScenarios(slug, filters);

  const [groupDialogOpen, setGroupDialogOpen] = useState(false);
  const [renaming, setRenaming] = useState<ScenarioGroup | null>(null);
  const [deletingGroup, setDeletingGroup] = useState<ScenarioGroup | null>(null);

  const setFilters = useCallback(
    (patch: Partial<ScenarioFilters>) => {
      const next = filtersToQueryString({ ...filtersFromSearchParams(new URLSearchParams(window.location.search)), ...patch });
      router.replace(next ? `${pathname}?${next}` : pathname, { scroll: false });
    },
    [router, pathname],
  );

  const tags = useMemo(() => [...new Set(all.data?.flatMap((scenario) => scenario.tags) ?? [])].sort(), [all.data]);
  const ungroupedCount = all.data?.filter((scenario) => scenario.groupId === null).length;
  const hasAgents = (agents.data?.length ?? 0) > 0;

  const newParams = new URLSearchParams();
  if (filters.agentId) newParams.set("agentId", filters.agentId);
  if (filters.groupId && filters.groupId !== UNGROUPED) newParams.set("groupId", filters.groupId);
  const newHref = `/w/${slug}/scenarios/new${newParams.size > 0 ? `?${newParams}` : ""}`;
  const newButton =
    canAuthor && hasAgents ? (
      <Button asChild>
        <Link href={newHref}>
          <PlusIcon />
          New scenario
        </Link>
      </Button>
    ) : null;

  let body;
  if (all.isError || scenarios.isError) {
    const error = all.error ?? scenarios.error;
    body = (
      <EmptyState
        icon={ClipboardListIcon}
        title="Could not load scenarios"
        description={errorMessage(error)}
        action={
          <Button variant="outline" onClick={() => Promise.all([all.refetch(), scenarios.refetch()])}>
            <RotateCwIcon />
            Try again
          </Button>
        }
      />
    );
  } else if (all.data?.length === 0) {
    body = agents.data && !hasAgents ? (
      <EmptyState
        icon={BotIcon}
        title="Add an agent first"
        description="Scenarios test one agent each. Connect the agent you want to test, then come back to write its scenarios."
        action={
          <Button variant="outline" asChild>
            <Link href={sectionHref(slug, SECTIONS.agents)}>Open agents</Link>
          </Button>
        }
      />
    ) : (
      <EmptyState
        icon={ClipboardListIcon}
        title="No scenarios yet"
        description={
          canAuthor
            ? "Describe a conversation: who calls, what they want, the facts they know and how the answer is judged."
            : "Members of this workspace write the scenarios that test your agents."
        }
        action={newButton}
      />
    );
  } else if (scenarios.data?.length === 0) {
    body = (
      <EmptyState
        icon={SearchXIcon}
        title="No scenarios match"
        description="Try another group, agent, tag or search."
        className="min-h-60"
        action={
          <Button variant="outline" onClick={() => setFilters(NO_FILTERS)}>
            Show all scenarios
          </Button>
        }
      />
    );
  } else {
    body = (
      <ScenarioList
        slug={slug}
        scenarios={scenarios.data}
        agents={agents.data}
        personas={personas.data}
        groups={groups.data}
        loading={scenarios.isPending || all.isPending}
      />
    );
  }

  return (
    <PageContainer>
      <PageHeader
        title="Scenarios"
        description={SECTIONS.scenarios.description}
        actions={all.data && all.data.length > 0 ? newButton : null}
      />
      <div className="grid items-start gap-6 lg:grid-cols-[14rem_minmax(0,1fr)]">
        <GroupSidebar
          groups={groups.data}
          selected={filters.groupId}
          totalCount={all.data?.length}
          ungroupedCount={ungroupedCount}
          canAuthor={canAuthor}
          onSelect={(groupId) => setFilters({ groupId })}
          onCreate={() => {
            setRenaming(null);
            setGroupDialogOpen(true);
          }}
          onRename={(group) => {
            setRenaming(group);
            setGroupDialogOpen(true);
          }}
          onDelete={setDeletingGroup}
        />
        <div className="flex min-w-0 flex-col gap-4">
          {all.data?.length !== 0 && (
            <ScenarioToolbar filters={filters} agents={agents.data} tags={tags} onChange={setFilters} />
          )}
          {body}
        </div>
      </div>
      {canAuthor && (
        <>
          <GroupDialog
            slug={slug}
            open={groupDialogOpen}
            onOpenChange={setGroupDialogOpen}
            group={renaming}
            onSaved={(group) => {
              if (!renaming) setFilters({ groupId: group.id });
            }}
          />
          <DeleteGroupDialog
            slug={slug}
            group={deletingGroup}
            onOpenChange={(open) => !open && setDeletingGroup(null)}
            onDeleted={(group) => {
              if (filters.groupId === group.id) setFilters({ groupId: null });
            }}
          />
        </>
      )}
    </PageContainer>
  );
}
