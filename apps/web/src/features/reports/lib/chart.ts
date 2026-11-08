import type { ReportTrendDay } from "@/lib/api/types";

const DAY_MS = 24 * 60 * 60 * 1000;

function isoDay(date: Date): string {
  return date.toISOString().slice(0, 10);
}

/** One entry per day of the period (UTC), ending today; days without runs are empty. */
export function fillTrendDays(trend: readonly ReportTrendDay[], days: number, now: Date = new Date()): ReportTrendDay[] {
  const byDate = new Map(trend.map((day) => [day.date, day]));
  const end = Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate());
  const filled: ReportTrendDay[] = [];
  for (let offset = days - 1; offset >= 0; offset--) {
    const date = isoDay(new Date(end - offset * DAY_MS));
    filled.push(byDate.get(date) ?? { date, runs: 0, avgScore: null, passRate: null });
  }
  return filled;
}

const STEPS = [1, 1.5, 2, 4, 5, 8, 10];

/** Smallest readable axis top ≥ value (at least 1). */
export function niceMax(value: number): number {
  if (value <= 1) return 1;
  const magnitude = 10 ** Math.floor(Math.log10(value));
  const step = STEPS.find((candidate) => candidate * magnitude >= value) ?? 10;
  return step * magnitude;
}

export type TrendBar = { date: string; x: number; y: number; width: number; height: number };
export type TrendPoint = { x: number; y: number } | null;

export type TrendChart = {
  runsMax: number;
  slot: number;
  bars: TrendBar[];
  points: TrendPoint[];
  /** SVG path data of each continuous run of scored days. */
  segments: string[];
};

/**
 * Geometry of the trend: runs as bars (0..niceMax) in one panel, the average score
 * as a line (0..100) in a second panel that shares the x positions.
 */
export function buildTrendChart(
  days: readonly ReportTrendDay[],
  size: { width: number; barHeight: number; lineHeight: number },
): TrendChart {
  const slot = days.length > 0 ? size.width / days.length : size.width;
  const runsMax = niceMax(Math.max(0, ...days.map((day) => day.runs)));
  const bars = days.map((day, index) => {
    const height = (day.runs / runsMax) * size.barHeight;
    return { date: day.date, x: index * slot, y: size.barHeight - height, width: slot, height };
  });
  const points: TrendPoint[] = days.map((day, index) =>
    day.avgScore === null ? null : { x: index * slot + slot / 2, y: size.lineHeight - (day.avgScore / 100) * size.lineHeight },
  );
  const segments: string[] = [];
  let current: string[] = [];
  for (const point of points) {
    if (point === null) {
      if (current.length) segments.push(current.join(" "));
      current = [];
      continue;
    }
    current.push(`${current.length ? "L" : "M"}${round(point.x)},${round(point.y)}`);
  }
  if (current.length) segments.push(current.join(" "));
  return { runsMax, slot, bars, points, segments };
}

function round(value: number): number {
  return Math.round(value * 100) / 100;
}
