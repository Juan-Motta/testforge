"use client";

import { keepPreviousData, useQuery } from "@tanstack/react-query";

import { api, unwrap } from "@/lib/api/client";

import { reportFiltersToApiQuery, type ReportFilters } from "../lib/filters";

export const reportsQueryKey = (slug: string) => ["workspaces", slug, "reports"] as const;

/** Aggregated results for the period and filters. */
export function useReportOverview(slug: string, filters: ReportFilters) {
  const query = reportFiltersToApiQuery(filters);
  return useQuery({
    queryKey: [...reportsQueryKey(slug), "overview", query],
    queryFn: () =>
      unwrap(api.GET("/api/v1/workspaces/{slug}/reports/overview", { params: { path: { slug }, query } })),
    placeholderData: keepPreviousData,
  });
}
