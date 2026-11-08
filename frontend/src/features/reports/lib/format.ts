const dayFormat = new Intl.DateTimeFormat("en", { month: "short", day: "numeric", timeZone: "UTC" });

/** 0–1 rate as a whole percentage. */
export function formatRate(rate: number | null): string {
  return rate === null ? "—" : `${Math.round(rate * 100)}%`;
}

export function formatAverage(value: number | null): string {
  return value === null ? "—" : String(Math.round(value));
}

/** `2026-11-07` → "Nov 7" (the day itself, whatever the viewer's time zone). */
export function formatDay(isoDay: string): string {
  return dayFormat.format(new Date(`${isoDay}T00:00:00Z`));
}

/** Show every n-th day label so long periods stay readable. */
export function axisLabelEvery(days: number): number {
  if (days <= 14) return 1;
  return Math.ceil(days / 6);
}

export function totalRuns(totals: Record<string, number>): number {
  return Object.values(totals).reduce((sum, count) => sum + count, 0);
}

export type RankedTag = { tag: string; count: number; share: number };

/** Most frequent first (ties by name); `share` is relative to the top tag. */
export function rankTags(tags: Record<string, number>): RankedTag[] {
  const entries = Object.entries(tags).sort(([a, ca], [b, cb]) => cb - ca || a.localeCompare(b));
  const top = entries[0]?.[1] ?? 0;
  return entries.map(([tag, count]) => ({ tag, count, share: top > 0 ? count / top : 0 }));
}
