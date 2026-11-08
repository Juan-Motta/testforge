import { describe, expect, it } from "vitest";

import { axisLabelEvery, formatAverage, formatDay, formatRate, rankTags, totalRuns } from "./format";

describe("report formatting", () => {
  it("formats rates as whole percentages and empties as a dash", () => {
    expect(formatRate(0.756)).toBe("76%");
    expect(formatRate(0)).toBe("0%");
    expect(formatRate(1)).toBe("100%");
    expect(formatRate(null)).toBe("—");
  });

  it("rounds averages", () => {
    expect(formatAverage(81.6)).toBe("82");
    expect(formatAverage(null)).toBe("—");
  });

  it("formats an ISO day without shifting time zones", () => {
    expect(formatDay("2026-11-07")).toBe("Nov 7");
    expect(formatDay("2026-01-01")).toBe("Jan 1");
  });

  it("thins x-axis labels for long periods", () => {
    expect(axisLabelEvery(7)).toBe(1);
    expect(axisLabelEvery(30)).toBe(5);
    expect(axisLabelEvery(90)).toBe(15);
  });

  it("sums runs across statuses", () => {
    expect(totalRuns({ completed: 3, failed: 1 })).toBe(4);
    expect(totalRuns({})).toBe(0);
  });

  it("ranks failure tags by count, then name, with a share of the top tag", () => {
    expect(rankTags({ slow_response: 2, off_topic: 5, incomplete_task: 2 })).toEqual([
      { tag: "off_topic", count: 5, share: 1 },
      { tag: "incomplete_task", count: 2, share: 0.4 },
      { tag: "slow_response", count: 2, share: 0.4 },
    ]);
    expect(rankTags({})).toEqual([]);
  });
});
