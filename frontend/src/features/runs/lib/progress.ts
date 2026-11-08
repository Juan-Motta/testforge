import type { RunBatch } from "@/lib/api/types";

export type BatchProgress = {
  finished: number;
  total: number;
  /** Finished runs against the total, 0–100. */
  percent: number;
  /** Queued or running runs. */
  active: number;
  /** Failed or interrupted runs. */
  failed: number;
  done: boolean;
};

export function batchProgress(batch: Pick<RunBatch, "total" | "counts">): BatchProgress {
  const count = (status: string) => batch.counts[status] ?? 0;
  const active = count("queued") + count("running");
  const finished = count("completed") + count("failed") + count("cancelled") + count("interrupted");
  // Reruns add runs beyond the launched total.
  const total = Math.max(batch.total, finished + active);
  const percent = total === 0 ? 0 : Math.min(100, Math.round((finished / total) * 100));
  return { finished, total, percent, active, failed: count("failed") + count("interrupted"), done: active === 0 };
}
