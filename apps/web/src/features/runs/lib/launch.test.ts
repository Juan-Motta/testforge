import { describe, expect, it } from "vitest";

import { buildLaunchRequest, launchSummary, mapLaunchErrors } from "./launch";

const scenarios = [
  { id: "s1", name: "Reschedule" },
  { id: "s2", name: "Refund" },
  { id: "s3", name: "Late fee" },
];

describe("launchSummary", () => {
  it("multiplies scenarios by repetitions", () => {
    expect(launchSummary(3, 2)).toBe("3 scenarios × 2 = 6 runs");
  });

  it("uses singular words for one", () => {
    expect(launchSummary(1, 1)).toBe("1 scenario × 1 = 1 run");
  });
});

describe("buildLaunchRequest", () => {
  it("sends ids, numbers and a trimmed label", () => {
    expect(buildLaunchRequest(["s1", "s2"], { repetitions: 2, concurrency: 4, label: "  nightly " })).toEqual({
      scenarioIds: ["s1", "s2"],
      repetitions: 2,
      concurrency: 4,
      label: "nightly",
    });
  });

  it("omits an empty label", () => {
    expect(buildLaunchRequest(["s1"], { repetitions: 1, concurrency: 3, label: "  " })).toEqual({
      scenarioIds: ["s1"],
      repetitions: 1,
      concurrency: 3,
    });
  });
});

describe("mapLaunchErrors", () => {
  it("names the scenario of each indexed error, in launch order", () => {
    const result = mapLaunchErrors(
      {
        "scenarioIds.2": "add a simulator connection and model",
        "scenarioIds.0": "the persona has no voice",
      },
      ["s1", "s2", "s3"],
      scenarios,
    );
    expect(result.scenarios).toEqual([
      { scenarioId: "s1", name: "Reschedule", message: "the persona has no voice", fix: "persona" },
      { scenarioId: "s3", name: "Late fee", message: "add a simulator connection and model", fix: "scenario" },
    ]);
    expect(result.fields).toEqual({});
    expect(result.other).toEqual([]);
  });

  it("keeps form fields apart and collects anything else", () => {
    const result = mapLaunchErrors(
      { repetitions: "Use 1 to 10.", label: "Too long", scenarioIds: "Pick at least one", "scenarioIds.9": "Unknown" },
      ["s1"],
      scenarios,
    );
    expect(result.fields).toEqual({ repetitions: "Use 1 to 10.", label: "Too long" });
    expect(result.scenarios).toEqual([]);
    expect(result.other).toEqual(["Pick at least one", "Unknown"]);
  });

  it("falls back to a generic name for a scenario it does not know", () => {
    const result = mapLaunchErrors({ "scenarioIds.0": "realtime is not supported yet" }, ["zz"], scenarios);
    expect(result.scenarios).toEqual([
      { scenarioId: "zz", name: "Scenario 1", message: "realtime is not supported yet", fix: "scenario" },
    ]);
  });
});
