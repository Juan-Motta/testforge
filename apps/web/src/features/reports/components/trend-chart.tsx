"use client";

import { useState } from "react";

import type { ReportTrendDay } from "@/lib/api/types";

import { buildTrendChart } from "../lib/chart";
import { axisLabelEvery, formatAverage, formatDay, formatRate } from "../lib/format";
import { useElementWidth } from "./use-element-width";

const GUTTER = 36;
const PANEL = 88;
const PANEL_GAP = 20;
const AXIS = 22;
const TITLE = 24;
const MAX_BAR = 28;

/** Rounded top, square base anchored on the baseline. */
function barPath(x: number, y: number, width: number, height: number): string {
  if (height <= 0) return "";
  const r = Math.min(4, width / 2, height);
  return `M${x},${y + height} V${y + r} Q${x},${y} ${x + r},${y} H${x + width - r} Q${x + width},${y} ${x + width},${y + r} V${y + height} Z`;
}

/**
 * Runs per day (bars) and average score per day (line, 0–100) as two stacked
 * panels sharing the day axis — one y-scale per panel, never a dual axis.
 */
export function TrendChart({ days }: { days: ReportTrendDay[] }) {
  const [container, width] = useElementWidth<HTMLDivElement>(640);
  const [hover, setHover] = useState<number | null>(null);
  const plotWidth = Math.max(120, width - GUTTER);
  const chart = buildTrendChart(days, { width: plotWidth, barHeight: PANEL, lineHeight: PANEL });
  const barsTop = TITLE;
  const lineTop = TITLE + PANEL + PANEL_GAP + TITLE;
  const height = lineTop + PANEL + AXIS;
  const every = axisLabelEvery(days.length);
  const barWidth = Math.max(1, Math.min(MAX_BAR, chart.slot - 2));
  const hovered = hover === null ? null : days[hover];

  return (
    <div ref={container} className="relative w-full">
      <svg
        width={width}
        height={height}
        viewBox={`0 0 ${width} ${height}`}
        className="block max-w-full overflow-visible"
        aria-hidden
        onMouseLeave={() => setHover(null)}
      >
        {/* Panel titles */}
        <text x={0} y={11} className="fill-muted-foreground text-[11px] font-medium">
          Runs
        </text>
        <text x={0} y={lineTop - TITLE + 11} className="fill-muted-foreground text-[11px] font-medium">
          Average score
        </text>

        {/* Runs panel */}
        <g transform={`translate(${GUTTER},${barsTop})`}>
          {[0, chart.runsMax / 2, chart.runsMax].map((tick) => {
            const y = PANEL - (tick / chart.runsMax) * PANEL;
            return (
              <g key={tick}>
                <line x1={0} x2={plotWidth} y1={y} y2={y} className="stroke-border" strokeWidth={1} />
                <text x={-6} y={y + 3.5} textAnchor="end" className="fill-muted-foreground text-[10px] tabular-nums">
                  {Number.isInteger(tick) ? tick : ""}
                </text>
              </g>
            );
          })}
          {chart.bars.map((bar, index) => (
            <path
              key={bar.date}
              d={barPath(bar.x + (bar.width - barWidth) / 2, bar.y, barWidth, bar.height)}
              className={hover === null || hover === index ? "fill-brand" : "fill-brand/45"}
            />
          ))}
        </g>

        {/* Average score panel */}
        <g transform={`translate(${GUTTER},${lineTop})`}>
          {[0, 50, 100].map((tick) => {
            const y = PANEL - (tick / 100) * PANEL;
            return (
              <g key={tick}>
                <line x1={0} x2={plotWidth} y1={y} y2={y} className="stroke-border" strokeWidth={1} />
                <text x={-6} y={y + 3.5} textAnchor="end" className="fill-muted-foreground text-[10px] tabular-nums">
                  {tick}
                </text>
              </g>
            );
          })}
          {chart.segments.map((d) => (
            <path key={d} d={d} fill="none" className="stroke-brand" strokeWidth={2} strokeLinejoin="round" strokeLinecap="round" />
          ))}
          {chart.points.map((point, index) =>
            point ? (
              <circle
                key={days[index].date}
                cx={point.x}
                cy={point.y}
                r={hover === index ? 5 : 4}
                className="fill-brand stroke-card"
                strokeWidth={2}
              />
            ) : null,
          )}
          {/* Day axis */}
          {/* Labels are anchored on the last day (today) and thinned for long periods. */}
          {days.map((day, index) =>
            (days.length - 1 - index) % every === 0 ? (
              <text
                key={day.date}
                x={index * chart.slot + chart.slot / 2}
                y={PANEL + 15}
                textAnchor="middle"
                className="fill-muted-foreground text-[10px]"
              >
                {formatDay(day.date)}
              </text>
            ) : null,
          )}
        </g>

        {/* Crosshair and hit targets (one per day, taller than the marks) */}
        <g transform={`translate(${GUTTER},0)`}>
          {hover !== null && (
            <line
              x1={hover * chart.slot + chart.slot / 2}
              x2={hover * chart.slot + chart.slot / 2}
              y1={barsTop}
              y2={lineTop + PANEL}
              className="stroke-muted-foreground/50"
              strokeDasharray="3 3"
            />
          )}
          {days.map((day, index) => (
            <rect
              key={day.date}
              x={index * chart.slot}
              y={0}
              width={chart.slot}
              height={lineTop + PANEL}
              fill="transparent"
              onMouseEnter={() => setHover(index)}
            />
          ))}
        </g>
      </svg>

      {hovered && hover !== null && (
        <div
          className="pointer-events-none absolute top-0 z-10 flex min-w-36 flex-col gap-0.5 rounded-lg border bg-popover px-3 py-2 text-xs text-popover-foreground shadow-md"
          style={{
            left: Math.min(Math.max(0, GUTTER + hover * chart.slot + chart.slot / 2 - 72), Math.max(0, width - 150)),
          }}
        >
          <span className="font-medium">{formatDay(hovered.date)}</span>
          <span className="flex justify-between gap-4 text-muted-foreground">
            Runs <span className="text-foreground tabular-nums">{hovered.runs}</span>
          </span>
          <span className="flex justify-between gap-4 text-muted-foreground">
            Average score <span className="text-foreground tabular-nums">{formatAverage(hovered.avgScore)}</span>
          </span>
          <span className="flex justify-between gap-4 text-muted-foreground">
            Pass rate <span className="text-foreground tabular-nums">{formatRate(hovered.passRate)}</span>
          </span>
        </div>
      )}

      <table className="sr-only">
        <caption>Runs, average score and pass rate per day</caption>
        <thead>
          <tr>
            <th scope="col">Day</th>
            <th scope="col">Runs</th>
            <th scope="col">Average score</th>
            <th scope="col">Pass rate</th>
          </tr>
        </thead>
        <tbody>
          {days.map((day) => (
            <tr key={day.date}>
              <th scope="row">{formatDay(day.date)}</th>
              <td>{day.runs}</td>
              <td>{formatAverage(day.avgScore)}</td>
              <td>{formatRate(day.passRate)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
