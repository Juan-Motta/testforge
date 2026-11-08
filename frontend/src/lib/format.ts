const dateFormat = new Intl.DateTimeFormat("en", { dateStyle: "medium" });
const dateTimeFormat = new Intl.DateTimeFormat("en", {
  dateStyle: "medium",
  timeStyle: "short",
});
const relativeFormat = new Intl.RelativeTimeFormat("en", { numeric: "auto" });

const UNITS: [Intl.RelativeTimeFormatUnit, number][] = [
  ["year", 365 * 24 * 60 * 60],
  ["month", 30 * 24 * 60 * 60],
  ["week", 7 * 24 * 60 * 60],
  ["day", 24 * 60 * 60],
  ["hour", 60 * 60],
  ["minute", 60],
];

export function formatDate(value: string): string {
  return dateFormat.format(new Date(value));
}

export function formatDateTime(value: string): string {
  return dateTimeFormat.format(new Date(value));
}

/** "3 days ago", "in 2 weeks", "just now". */
export function formatRelative(value: string, now: Date = new Date()): string {
  const seconds = Math.round((new Date(value).getTime() - now.getTime()) / 1000);
  for (const [unit, size] of UNITS) {
    if (Math.abs(seconds) >= size) {
      return relativeFormat.format(Math.trunc(seconds / size), unit);
    }
  }
  return "just now";
}
