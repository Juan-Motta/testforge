import type { RunStatus } from "@/lib/api/types";

import { isRunStatus } from "./status";

/** Filters of the runs list, mirrored in the URL query string. */
export type RunFilters = {
  batchId: string | null;
  status: RunStatus | null;
  agentId: string | null;
};

export const NO_RUN_FILTERS: RunFilters = { batchId: null, status: null, agentId: null };

type ParamsLike = { get(name: string): string | null };

export function filtersFromSearchParams(params: ParamsLike): RunFilters {
  const status = params.get("status");
  return {
    batchId: params.get("batchId") || null,
    status: isRunStatus(status) ? status : null,
    agentId: params.get("agentId") || null,
  };
}

/** Query object with only the filters that are set (also the API query). */
export type RunApiQuery = { batchId?: string; status?: RunStatus; agentId?: string };

export function filtersToApiQuery(filters: RunFilters): RunApiQuery {
  const query: RunApiQuery = {};
  if (filters.batchId) query.batchId = filters.batchId;
  if (filters.status) query.status = filters.status;
  if (filters.agentId) query.agentId = filters.agentId;
  return query;
}

export function filtersToQueryString(filters: RunFilters): string {
  return new URLSearchParams(filtersToApiQuery(filters)).toString();
}
