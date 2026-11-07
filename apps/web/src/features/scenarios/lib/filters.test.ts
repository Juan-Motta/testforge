import { describe, expect, it } from "vitest";

import { filtersFromSearchParams, filtersToApiQuery, filtersToQueryString, type ScenarioFilters } from "./filters";

describe("scenario filters", () => {
  it("reads every filter from the query string", () => {
    const params = new URLSearchParams("groupId=none&agentId=a1&tag=billing&q=refund");
    expect(filtersFromSearchParams(params)).toEqual({ groupId: "none", agentId: "a1", tag: "billing", q: "refund" });
  });

  it("defaults to all scenarios with no filters", () => {
    expect(filtersFromSearchParams(new URLSearchParams())).toEqual({ groupId: null, agentId: null, tag: null, q: "" });
  });

  it("treats empty values as absent", () => {
    expect(filtersFromSearchParams(new URLSearchParams("groupId=&agentId=&tag=&q="))).toEqual({
      groupId: null,
      agentId: null,
      tag: null,
      q: "",
    });
  });

  it("writes only the filters that are set, trimming the search", () => {
    const filters: ScenarioFilters = { groupId: "g1", agentId: null, tag: "vip", q: "  late fee " };
    expect(filtersToQueryString(filters)).toBe("groupId=g1&tag=vip&q=late+fee");
    expect(filtersToQueryString({ groupId: null, agentId: null, tag: null, q: "  " })).toBe("");
  });

  it("round-trips through the URL", () => {
    const filters: ScenarioFilters = { groupId: "none", agentId: "a 1", tag: "café & co", q: "x" };
    expect(filtersFromSearchParams(new URLSearchParams(filtersToQueryString(filters)))).toEqual(filters);
  });

  it("builds the API query without empty filters", () => {
    expect(filtersToApiQuery({ groupId: "none", agentId: null, tag: null, q: " a " })).toEqual({ groupId: "none", q: "a" });
    expect(filtersToApiQuery({ groupId: null, agentId: null, tag: null, q: "" })).toEqual({});
  });
});
