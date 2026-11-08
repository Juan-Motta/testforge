import { describe, expect, it } from "vitest";

import {
  filtersFromSearchParams,
  filtersToApiQuery,
  filtersToQueryString,
  hasRunFilters,
  NO_RUN_FILTERS,
  type RunFilters,
} from "./filters";

describe("run filters", () => {
  it("reads batch, status and agent from the query string", () => {
    expect(filtersFromSearchParams(new URLSearchParams("batchId=b1&status=running&agentId=a1"))).toEqual({
      batchId: "b1",
      status: "running",
      agentId: "a1",
      scenarioId: null,
    });
  });

  it("ignores empty values and unknown statuses", () => {
    expect(filtersFromSearchParams(new URLSearchParams("batchId=&status=bogus&agentId="))).toEqual({
      batchId: null,
      status: null,
      agentId: null,
      scenarioId: null,
    });
  });

  it("writes only the filters that are set and round-trips", () => {
    const filters: RunFilters = { batchId: "b 1", status: "failed", agentId: null, scenarioId: null };
    expect(filtersToQueryString(filters)).toBe("batchId=b+1&status=failed");
    expect(filtersFromSearchParams(new URLSearchParams(filtersToQueryString(filters)))).toEqual(filters);
    expect(filtersToQueryString(NO_RUN_FILTERS)).toBe("");
  });

  it("builds the API query", () => {
    expect(filtersToApiQuery({ batchId: null, status: "queued", agentId: "a1", scenarioId: null })).toEqual({ status: "queued", agentId: "a1" });
  });
});

describe("scenario filter", () => {
  it("reads, writes and sends the scenario", () => {
    const filters = filtersFromSearchParams(new URLSearchParams("scenarioId=s1"));
    expect(filters).toEqual({ ...NO_RUN_FILTERS, scenarioId: "s1" });
    expect(filtersToQueryString(filters)).toBe("scenarioId=s1");
    expect(filtersToApiQuery(filters)).toEqual({ scenarioId: "s1" });
  });

  it("counts as filtered", () => {
    expect(hasRunFilters(NO_RUN_FILTERS)).toBe(false);
    expect(hasRunFilters({ ...NO_RUN_FILTERS, scenarioId: "s1" })).toBe(true);
    expect(hasRunFilters({ ...NO_RUN_FILTERS, status: "failed" })).toBe(true);
  });
});
