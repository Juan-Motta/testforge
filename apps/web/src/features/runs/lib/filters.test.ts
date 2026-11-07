import { describe, expect, it } from "vitest";

import { filtersFromSearchParams, filtersToApiQuery, filtersToQueryString, type RunFilters } from "./filters";

describe("run filters", () => {
  it("reads batch, status and agent from the query string", () => {
    expect(filtersFromSearchParams(new URLSearchParams("batchId=b1&status=running&agentId=a1"))).toEqual({
      batchId: "b1",
      status: "running",
      agentId: "a1",
    });
  });

  it("ignores empty values and unknown statuses", () => {
    expect(filtersFromSearchParams(new URLSearchParams("batchId=&status=bogus&agentId="))).toEqual({
      batchId: null,
      status: null,
      agentId: null,
    });
  });

  it("writes only the filters that are set and round-trips", () => {
    const filters: RunFilters = { batchId: "b 1", status: "failed", agentId: null };
    expect(filtersToQueryString(filters)).toBe("batchId=b+1&status=failed");
    expect(filtersFromSearchParams(new URLSearchParams(filtersToQueryString(filters)))).toEqual(filters);
    expect(filtersToQueryString({ batchId: null, status: null, agentId: null })).toBe("");
  });

  it("builds the API query", () => {
    expect(filtersToApiQuery({ batchId: null, status: "queued", agentId: "a1" })).toEqual({ status: "queued", agentId: "a1" });
  });
});
