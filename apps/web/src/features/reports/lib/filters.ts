/** Report filters, mirrored in the URL query string. */
export const REPORT_PERIODS = [7, 30, 90] as const;
export type ReportPeriod = (typeof REPORT_PERIODS)[number];

export type ReportFilters = {
  days: ReportPeriod;
  agentId: string | null;
  scenarioId: string | null;
};

export const NO_REPORT_FILTERS: ReportFilters = { days: 7, agentId: null, scenarioId: null };

type ParamsLike = { get(name: string): string | null };

function toPeriod(value: string | null): ReportPeriod {
  const days = Number(value);
  return (REPORT_PERIODS as readonly number[]).includes(days) ? (days as ReportPeriod) : NO_REPORT_FILTERS.days;
}

export function reportFiltersFromSearchParams(params: ParamsLike): ReportFilters {
  return {
    days: toPeriod(params.get("days")),
    agentId: params.get("agentId") || null,
    scenarioId: params.get("scenarioId") || null,
  };
}

export type ReportApiQuery = { days: ReportPeriod; agentId?: string; scenarioId?: string };

export function reportFiltersToApiQuery(filters: ReportFilters): ReportApiQuery {
  const query: ReportApiQuery = { days: filters.days };
  if (filters.agentId) query.agentId = filters.agentId;
  if (filters.scenarioId) query.scenarioId = filters.scenarioId;
  return query;
}

/** Query string without the default period. */
export function reportFiltersToQueryString(filters: ReportFilters): string {
  const params = new URLSearchParams();
  if (filters.days !== NO_REPORT_FILTERS.days) params.set("days", String(filters.days));
  if (filters.agentId) params.set("agentId", filters.agentId);
  if (filters.scenarioId) params.set("scenarioId", filters.scenarioId);
  return params.toString();
}
