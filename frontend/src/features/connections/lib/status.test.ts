import { describe, expect, it } from "vitest";

import { describeLastTest } from "./status";

const now = new Date("2026-11-07T12:00:00Z");

describe("describeLastTest", () => {
  it("reports a connection that was never tested", () => {
    expect(describeLastTest(null, now)).toEqual({ state: "never", label: "Never tested" });
  });

  it("reports a successful test with latency and age", () => {
    expect(
      describeLastTest(
        { ok: true, message: "OK", latencyMs: 312, testedAt: "2026-11-07T11:58:00Z" },
        now,
      ),
    ).toEqual({ state: "ok", label: "Connected · 312 ms · 2 minutes ago" });
  });

  it("reports a failure with the server message", () => {
    expect(
      describeLastTest(
        { ok: false, message: "Invalid API key", latencyMs: 90, testedAt: "2026-11-07T12:00:00Z" },
        now,
      ),
    ).toEqual({ state: "failed", label: "Failed: Invalid API key" });
  });
});
