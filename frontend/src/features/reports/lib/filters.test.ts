import { describe, expect, it } from "vitest";

import { NO_REPORT_FILTERS, reportFiltersFromSearchParams, reportFiltersToApiQuery, reportFiltersToQueryString } from "./filters";

describe("report filters", () => {
  it("defaults to 7 days without agent or scenario", () => {
    expect(reportFiltersFromSearchParams(new URLSearchParams(""))).toEqual(NO_REPORT_FILTERS);
    expect(NO_REPORT_FILTERS).toEqual({ days: 7, agentId: null, scenarioId: null });
  });

  it("reads days, agent and scenario, ignoring unsupported periods", () => {
    expect(reportFiltersFromSearchParams(new URLSearchParams("days=30&agentId=a1&scenarioId=s1"))).toEqual({
      days: 30,
      agentId: "a1",
      scenarioId: "s1",
    });
    expect(reportFiltersFromSearchParams(new URLSearchParams("days=12&agentId=")).days).toBe(7);
  });

  it("omits the default period from the URL and round-trips", () => {
    expect(reportFiltersToQueryString(NO_REPORT_FILTERS)).toBe("");
    const filters = { days: 90 as const, agentId: null, scenarioId: "s 1" };
    expect(reportFiltersToQueryString(filters)).toBe("days=90&scenarioId=s+1");
    expect(reportFiltersFromSearchParams(new URLSearchParams(reportFiltersToQueryString(filters)))).toEqual(filters);
  });

  it("always sends days to the API", () => {
    expect(reportFiltersToApiQuery(NO_REPORT_FILTERS)).toEqual({ days: 7 });
    expect(reportFiltersToApiQuery({ days: 30, agentId: "a", scenarioId: null })).toEqual({ days: 30, agentId: "a" });
  });
});
