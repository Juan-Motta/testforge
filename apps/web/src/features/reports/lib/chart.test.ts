import { describe, expect, it } from "vitest";

import { buildTrendChart, fillTrendDays, niceMax } from "./chart";

describe("fillTrendDays", () => {
  it("returns one entry per day ending today, filling missing days with zero runs", () => {
    const days = fillTrendDays(
      [{ date: "2026-11-05", runs: 3, avgScore: 80, passRate: 0.5 }],
      3,
      new Date("2026-11-07T10:00:00Z"),
    );
    expect(days).toEqual([
      { date: "2026-11-05", runs: 3, avgScore: 80, passRate: 0.5 },
      { date: "2026-11-06", runs: 0, avgScore: null, passRate: null },
      { date: "2026-11-07", runs: 0, avgScore: null, passRate: null },
    ]);
  });
});

describe("niceMax", () => {
  it("rounds the axis top up to a readable step", () => {
    expect(niceMax(0)).toBe(1);
    expect(niceMax(1)).toBe(1);
    expect(niceMax(3)).toBe(4);
    expect(niceMax(7)).toBe(8);
    expect(niceMax(13)).toBe(15);
    expect(niceMax(42)).toBe(50);
  });
});

describe("buildTrendChart", () => {
  const days = [
    { date: "d1", runs: 2, avgScore: 100, passRate: 1 },
    { date: "d2", runs: 0, avgScore: null, passRate: null },
    { date: "d3", runs: 4, avgScore: 50, passRate: 0.5 },
    { date: "d4", runs: 1, avgScore: 0, passRate: 0 },
  ];
  const chart = buildTrendChart(days, { width: 400, barHeight: 100, lineHeight: 100 });

  it("splits the width into equal slots and scales bars to the max runs", () => {
    expect(chart.runsMax).toBe(4);
    expect(chart.bars).toHaveLength(4);
    expect(chart.bars[0]).toMatchObject({ x: 0, width: 100, height: 50, y: 50 });
    expect(chart.bars[1].height).toBe(0);
    expect(chart.bars[2]).toMatchObject({ height: 100, y: 0 });
  });

  it("places score points at slot centres on a 0-100 scale and breaks the line on empty days", () => {
    expect(chart.points.map((point) => point && [point.x, point.y])).toEqual([[50, 0], null, [250, 50], [350, 100]]);
    expect(chart.segments).toEqual(["M50,0", "M250,50 L350,100"]);
  });

  it("handles an all-empty period", () => {
    const empty = buildTrendChart([{ date: "d1", runs: 0, avgScore: null, passRate: null }], {
      width: 100,
      barHeight: 50,
      lineHeight: 50,
    });
    expect(empty.runsMax).toBe(1);
    expect(empty.bars[0].height).toBe(0);
    expect(empty.segments).toEqual([]);
  });
});
