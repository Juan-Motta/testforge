"use client";

import { XIcon } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import type { Agent, RunBatch, Scenario } from "@/lib/api/types";

import { hasRunFilters, NO_RUN_FILTERS, type RunFilters } from "../lib/filters";
import { RUN_STATUSES, statusBadge } from "../lib/status";
import { batchTitle } from "./batch-list";

const ALL = "all";

type RunsToolbarProps = {
  filters: RunFilters;
  batches: RunBatch[] | undefined;
  agents: Agent[] | undefined;
  scenarios: Scenario[] | undefined;
  onChange: (patch: Partial<RunFilters>) => void;
};

export function RunsToolbar({ filters, batches, agents, scenarios, onChange }: RunsToolbarProps) {
  const filtered = hasRunFilters(filters);
  const scenarioKnown = filters.scenarioId === null || scenarios?.some((scenario) => scenario.id === filters.scenarioId);
  const batchKnown = filters.batchId === null || batches?.some((batch) => batch.id === filters.batchId);
  return (
    <div className="flex flex-wrap items-center gap-2">
      <Select value={filters.batchId ?? ALL} onValueChange={(value) => onChange({ batchId: value === ALL ? null : value })}>
        <SelectTrigger className="w-52" aria-label="Filter by batch">
          <SelectValue placeholder="All batches" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={ALL}>All batches</SelectItem>
          {/* Keep an older batch (beyond the recent list) selectable. */}
          {!batchKnown && filters.batchId && <SelectItem value={filters.batchId}>Selected batch</SelectItem>}
          {batches?.map((batch) => (
            <SelectItem key={batch.id} value={batch.id}>
              {batchTitle(batch)}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Select
        value={filters.status ?? ALL}
        onValueChange={(value) => onChange({ status: value === ALL ? null : (value as RunFilters["status"]) })}
      >
        <SelectTrigger className="w-36" aria-label="Filter by status">
          <SelectValue placeholder="All statuses" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={ALL}>All statuses</SelectItem>
          {RUN_STATUSES.map((status) => (
            <SelectItem key={status} value={status}>
              {statusBadge(status).label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Select value={filters.agentId ?? ALL} onValueChange={(value) => onChange({ agentId: value === ALL ? null : value })}>
        <SelectTrigger className="w-44" aria-label="Filter by agent">
          <SelectValue placeholder="All agents" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={ALL}>All agents</SelectItem>
          {agents?.map((agent) => (
            <SelectItem key={agent.id} value={agent.id}>
              {agent.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Select
        value={filters.scenarioId ?? ALL}
        onValueChange={(value) => onChange({ scenarioId: value === ALL ? null : value })}
      >
        <SelectTrigger className="w-48" aria-label="Filter by scenario">
          <SelectValue placeholder="All scenarios" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={ALL}>All scenarios</SelectItem>
          {!scenarioKnown && filters.scenarioId && <SelectItem value={filters.scenarioId}>Selected scenario</SelectItem>}
          {scenarios?.map((scenario) => (
            <SelectItem key={scenario.id} value={scenario.id}>
              {scenario.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      {filtered && (
        <Button type="button" variant="ghost" size="sm" onClick={() => onChange(NO_RUN_FILTERS)}>
          <XIcon />
          Clear filters
        </Button>
      )}
    </div>
  );
}
