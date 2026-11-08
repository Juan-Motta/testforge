"use client";

import { SearchIcon, XIcon } from "lucide-react";
import { useEffect, useState } from "react";

import { Button } from "@/components/ui/button";
import { InputGroup, InputGroupAddon, InputGroupInput } from "@/components/ui/input-group";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import type { Agent } from "@/lib/api/types";

import type { ScenarioFilters } from "../lib/filters";

const ALL = "all";
const SEARCH_DELAY_MS = 300;

type ScenarioToolbarProps = {
  filters: ScenarioFilters;
  agents: Agent[] | undefined;
  tags: string[];
  onChange: (patch: Partial<ScenarioFilters>) => void;
};

export function ScenarioToolbar({ filters, agents, tags, onChange }: ScenarioToolbarProps) {
  // The search box is local while typing and reaches the URL after a short pause.
  const [search, setSearch] = useState(filters.q);
  const [urlQuery, setUrlQuery] = useState(filters.q);
  if (filters.q !== urlQuery) {
    // The URL changed (back/forward, clear): follow it unless it is what is being typed.
    setUrlQuery(filters.q);
    if (filters.q.trim() !== search.trim()) setSearch(filters.q);
  }

  useEffect(() => {
    if (search.trim() === filters.q.trim()) return;
    const timer = setTimeout(() => onChange({ q: search }), SEARCH_DELAY_MS);
    return () => clearTimeout(timer);
  }, [search, filters.q, onChange]);

  const filtered = filters.agentId !== null || filters.tag !== null || filters.q.trim() !== "";
  // Keep a filtered tag selectable even when no loaded scenario has it anymore.
  const tagOptions = filters.tag && !tags.includes(filters.tag) ? [filters.tag, ...tags] : tags;

  return (
    <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
      <InputGroup className="sm:max-w-xs">
        <InputGroupAddon>
          <SearchIcon aria-hidden />
        </InputGroupAddon>
        <InputGroupInput
          type="search"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          placeholder="Search name or goal"
          aria-label="Search scenarios"
          maxLength={200}
        />
      </InputGroup>
      <div className="flex flex-wrap items-center gap-2">
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
        <Select value={filters.tag ?? ALL} onValueChange={(value) => onChange({ tag: value === ALL ? null : value })}>
          <SelectTrigger className="w-36" aria-label="Filter by tag" disabled={tagOptions.length === 0}>
            <SelectValue placeholder="All tags" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL}>All tags</SelectItem>
            {tagOptions.map((tag) => (
              <SelectItem key={tag} value={tag}>
                {tag}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {filtered && (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => {
              setSearch("");
              onChange({ agentId: null, tag: null, q: "" });
            }}
          >
            <XIcon />
            Clear filters
          </Button>
        )}
      </div>
    </div>
  );
}
