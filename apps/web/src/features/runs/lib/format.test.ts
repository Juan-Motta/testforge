import { describe, expect, it } from "vitest";

import { formatCost, formatDuration, formatLatency, formatOffset, formatTokens, isSlowLatency, SLOW_LATENCY_MS } from "./format";

describe("formatLatency", () => {
  it("shows milliseconds under a second and seconds above", () => {
    expect(formatLatency(820)).toBe("820 ms");
    expect(formatLatency(1640)).toBe("1.64 s");
    expect(formatLatency(null)).toBe("—");
  });
});

describe("isSlowLatency", () => {
  it("flags latencies above 1500 ms", () => {
    expect(SLOW_LATENCY_MS).toBe(1500);
    expect(isSlowLatency(1500)).toBe(false);
    expect(isSlowLatency(1501)).toBe(true);
    expect(isSlowLatency(null)).toBe(false);
  });
});

describe("formatDuration", () => {
  it("formats seconds and minutes", () => {
    expect(formatDuration(4200)).toBe("4.2 s");
    expect(formatDuration(65_000)).toBe("1m 05s");
    expect(formatDuration(3_725_000)).toBe("62m 05s");
    expect(formatDuration(null)).toBe("—");
  });
});

describe("formatOffset", () => {
  it("formats a transcript offset as mm:ss", () => {
    expect(formatOffset(0)).toBe("00:00");
    expect(formatOffset(9_999)).toBe("00:09");
    expect(formatOffset(125_000)).toBe("02:05");
  });
});

describe("formatTokens and formatCost", () => {
  it("groups token counts", () => {
    expect(formatTokens(1234567)).toBe("1,234,567");
  });

  it("shows Langfuse cost in dollars with enough precision for small amounts", () => {
    expect(formatCost(null)).toBe("—");
    expect(formatCost(0)).toBe("$0.00");
    expect(formatCost(0.00421)).toBe("$0.0042");
    expect(formatCost(1.5)).toBe("$1.50");
  });
});
