"use client";

import { BotIcon, FolderIcon, ListChecksIcon, UserRoundIcon } from "lucide-react";
import Link from "next/link";

import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { ModalityBadge } from "@/features/agents/components/modality-badge";
import type { Agent, Persona, Scenario, ScenarioGroup } from "@/lib/api/types";

type ScenarioListProps = {
  slug: string;
  scenarios: Scenario[] | undefined;
  agents: Agent[] | undefined;
  personas: Persona[] | undefined;
  groups: ScenarioGroup[] | undefined;
  loading: boolean;
};

const MAX_TAGS = 3;

export function criteriaSummary(scenario: Pick<Scenario, "criteria">): string {
  const total = scenario.criteria.length;
  const must = scenario.criteria.filter((criterion) => criterion.kind === "must").length;
  return `${total === 1 ? "1 criterion" : `${total} criteria`} · ${must} must`;
}

function RowSkeleton() {
  return (
    <li className="flex flex-col gap-2 px-5 py-4">
      <Skeleton className="h-4 w-56" />
      <Skeleton className="h-3 w-80 max-w-full" />
    </li>
  );
}

export function ScenarioList({ slug, scenarios, agents, personas, groups, loading }: ScenarioListProps) {
  if (loading) {
    return (
      <Card className="gap-0 py-0">
        <ul className="divide-y" aria-busy aria-label="Loading scenarios">
          <RowSkeleton />
          <RowSkeleton />
          <RowSkeleton />
        </ul>
      </Card>
    );
  }

  return (
    <Card className="gap-0 py-0">
      <ul className="divide-y">
        {scenarios?.map((scenario) => {
          const agent = agents?.find((item) => item.id === scenario.agentId);
          const persona = personas?.find((item) => item.id === scenario.personaId);
          const group = scenario.groupId ? groups?.find((item) => item.id === scenario.groupId) : undefined;
          const extraTags = scenario.tags.length - MAX_TAGS;
          return (
            <li
              key={scenario.id}
              className="relative flex flex-col gap-2 px-5 py-4 transition-colors focus-within:bg-muted/30 hover:bg-muted/30 sm:flex-row sm:items-center sm:gap-6"
            >
              <div className="flex min-w-0 flex-1 flex-col gap-1.5">
                <h3 className="truncate text-sm font-medium">
                  <Link href={`/w/${slug}/scenarios/${scenario.id}`} className="outline-none after:absolute after:inset-0">
                    {scenario.name}
                  </Link>
                </h3>
                <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
                  <span className="inline-flex min-w-0 items-center gap-1.5">
                    <BotIcon aria-hidden className="size-3.5 shrink-0" />
                    <span className="truncate">{agent?.name ?? "Agent"}</span>
                    <ModalityBadge modality={scenario.modality} />
                  </span>
                  <span className="inline-flex min-w-0 items-center gap-1">
                    <UserRoundIcon aria-hidden className="size-3.5 shrink-0" />
                    <span className="truncate">{persona?.name ?? "Persona"}</span>
                  </span>
                  {group && (
                    <span className="inline-flex min-w-0 items-center gap-1">
                      <FolderIcon aria-hidden className="size-3.5 shrink-0" />
                      <span className="truncate">{group.name}</span>
                    </span>
                  )}
                </div>
              </div>
              <div className="flex shrink-0 flex-wrap items-center gap-2 text-xs text-muted-foreground sm:justify-end">
                {scenario.tags.slice(0, MAX_TAGS).map((tag) => (
                  <Badge key={tag} variant="outline" className="font-normal">
                    {tag}
                  </Badge>
                ))}
                {extraTags > 0 && <span>+{extraTags}</span>}
                <span className="inline-flex items-center gap-1 tabular-nums">
                  <ListChecksIcon aria-hidden className="size-3.5" />
                  {criteriaSummary(scenario)}
                </span>
              </div>
            </li>
          );
        })}
      </ul>
    </Card>
  );
}
