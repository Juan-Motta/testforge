import type { RunStatus } from "@/lib/api/types";

export const RUN_STATUSES: readonly RunStatus[] = [
  "queued",
  "running",
  "completed",
  "failed",
  "cancelled",
  "interrupted",
];

export type StatusTone = "neutral" | "brand" | "success" | "destructive" | "warning";

export type StatusBadgeInfo = {
  label: string;
  tone: StatusTone;
  /** Animated dot for runs that are still going. */
  pulse: boolean;
};

const BADGES: Record<RunStatus, StatusBadgeInfo> = {
  queued: { label: "Queued", tone: "neutral", pulse: true },
  running: { label: "Running", tone: "brand", pulse: true },
  completed: { label: "Completed", tone: "success", pulse: false },
  failed: { label: "Failed", tone: "destructive", pulse: false },
  cancelled: { label: "Cancelled", tone: "neutral", pulse: false },
  interrupted: { label: "Interrupted", tone: "warning", pulse: false },
};

export function statusBadge(status: RunStatus): StatusBadgeInfo {
  return BADGES[status];
}

export function isRunStatus(value: unknown): value is RunStatus {
  return typeof value === "string" && (RUN_STATUSES as readonly string[]).includes(value);
}

/** Queued or running: the run can still change. */
export function isActiveStatus(status: RunStatus): boolean {
  return status === "queued" || status === "running";
}

/** Lists refresh while any visible run can still change. */
export function shouldPoll(runs: readonly { status: RunStatus }[]): boolean {
  return runs.some((run) => isActiveStatus(run.status));
}

/** Human label of an end reason, e.g. `user_hung_up` → "User hung up". */
export function formatEndReason(reason: string | null): string {
  if (!reason) return "—";
  const words = reason.replaceAll("_", " ");
  return words.charAt(0).toUpperCase() + words.slice(1);
}
