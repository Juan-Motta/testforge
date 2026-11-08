/** Filters of the scenarios list, mirrored in the URL query string. */
export type ScenarioFilters = {
  /** A group id, `none` for ungrouped scenarios, or null for all of them. */
  groupId: string | null;
  agentId: string | null;
  tag: string | null;
  q: string;
};

export const NO_FILTERS: ScenarioFilters = { groupId: null, agentId: null, tag: null, q: "" };

/** Group filter value for scenarios without a group (as the API expects it). */
export const UNGROUPED = "none";

type ParamsLike = { get(name: string): string | null };

export function filtersFromSearchParams(params: ParamsLike): ScenarioFilters {
  const read = (name: string) => params.get(name) || null;
  return { groupId: read("groupId"), agentId: read("agentId"), tag: read("tag"), q: params.get("q") ?? "" };
}

/** Query object with only the filters that are set (also the API query). */
export function filtersToApiQuery(filters: ScenarioFilters): Partial<Record<keyof ScenarioFilters, string>> {
  const query: Partial<Record<keyof ScenarioFilters, string>> = {};
  if (filters.groupId) query.groupId = filters.groupId;
  if (filters.agentId) query.agentId = filters.agentId;
  if (filters.tag) query.tag = filters.tag;
  const q = filters.q.trim();
  if (q) query.q = q;
  return query;
}

export function filtersToQueryString(filters: ScenarioFilters): string {
  return new URLSearchParams(filtersToApiQuery(filters)).toString();
}
