import { describe, expect, it } from "vitest";

import { formatRelative } from "./format";

const now = new Date("2026-11-07T12:00:00Z");

describe("formatRelative", () => {
  it("says just now under a minute", () => {
    expect(formatRelative("2026-11-07T11:59:30Z", now)).toBe("just now");
  });

  it("formats past and future distances", () => {
    expect(formatRelative("2026-11-04T12:00:00Z", now)).toBe("3 days ago");
    expect(formatRelative("2026-11-21T12:00:00Z", now)).toBe("in 2 weeks");
    expect(formatRelative("2026-11-06T12:00:00Z", now)).toBe("yesterday");
  });
});
