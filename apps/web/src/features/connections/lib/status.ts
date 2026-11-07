import type { ConnectionTest } from "@/lib/api/types";

import { formatRelative } from "../../../lib/format";

export type LastTestState = "never" | "ok" | "failed";

/** "Never tested", "Connected · 312 ms · 2 minutes ago" or "Failed: <server message>". */
export function describeLastTest(
  lastTest: ConnectionTest | null,
  now: Date = new Date(),
): { state: LastTestState; label: string } {
  if (!lastTest) return { state: "never", label: "Never tested" };
  if (!lastTest.ok) return { state: "failed", label: `Failed: ${lastTest.message}` };
  return {
    state: "ok",
    label: `Connected · ${lastTest.latencyMs} ms · ${formatRelative(lastTest.testedAt, now)}`,
  };
}
