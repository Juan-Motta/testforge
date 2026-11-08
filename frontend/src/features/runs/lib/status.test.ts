import { describe, expect, it } from "vitest";

import { isActiveStatus, RUN_STATUSES, shouldPoll, statusBadge } from "./status";

describe("statusBadge", () => {
  it("labels every status and animates only active ones", () => {
    for (const status of RUN_STATUSES) {
      const badge = statusBadge(status);
      expect(badge.label.length).toBeGreaterThan(0);
      expect(badge.pulse).toBe(status === "queued" || status === "running");
    }
    expect(statusBadge("completed")).toMatchObject({ label: "Completed", tone: "success" });
    expect(statusBadge("failed")).toMatchObject({ label: "Failed", tone: "destructive" });
    expect(statusBadge("running")).toMatchObject({ label: "Running", tone: "brand" });
    expect(statusBadge("interrupted")).toMatchObject({ tone: "warning" });
  });
});

describe("isActiveStatus", () => {
  it("is true while queued or running", () => {
    expect(RUN_STATUSES.filter(isActiveStatus)).toEqual(["queued", "running"]);
  });
});

describe("shouldPoll", () => {
  it("polls while any run is queued or running", () => {
    expect(shouldPoll([{ status: "completed" }, { status: "queued" }])).toBe(true);
    expect(shouldPoll([{ status: "completed" }, { status: "failed" }])).toBe(false);
    expect(shouldPoll([])).toBe(false);
  });
});

describe("shouldPoll with judgements", () => {
  it("also polls while a visible run's judgement is pending", () => {
    expect(shouldPoll([{ status: "completed", judgementStatus: "waiting_traces" }])).toBe(true);
    expect(shouldPoll([{ status: "completed", judgementStatus: "queued" }])).toBe(true);
    expect(shouldPoll([{ status: "completed", judgementStatus: "completed" }])).toBe(false);
    expect(shouldPoll([{ status: "failed", judgementStatus: null }])).toBe(false);
  });
});
