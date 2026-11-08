import type { Agent, Capability, CriterionSource, Modality, SimulatorMode } from "@/lib/api/types";

/** Integer weights for `count` criteria that add up to exactly 100; the first ones take the remainder. */
export function distributeEvenly(count: number): number[] {
  if (count <= 0) return [];
  const base = Math.floor(100 / count);
  const remainder = 100 - base * count;
  return Array.from({ length: count }, (_, index) => base + (index < remainder ? 1 : 0));
}

export type WeightTotal = { total: number; status: "ok" | "under" | "over"; message: string };

/** Live total of the weight inputs (blank or invalid ones count as 0). */
export function weightTotal(weights: string[]): WeightTotal {
  const total = weights.reduce((sum, weight) => {
    const value = Number(weight);
    return sum + (weight.trim() !== "" && Number.isFinite(value) ? value : 0);
  }, 0);
  if (total === 100) return { total, status: "ok", message: "Weights total 100." };
  if (total < 100) return { total, status: "under", message: `Add ${100 - total} to reach 100.` };
  return { total, status: "over", message: `Remove ${total - 100} to get back to 100.` };
}

export const CRITERION_SOURCES: { value: CriterionSource; label: string }[] = [
  { value: "transcript", label: "Transcript" },
  { value: "trace", label: "Trace" },
  { value: "timing", label: "Timing" },
];

export type SourceAvailability = { available: boolean; reason?: string };

/** Which evidence sources the judge can use for the agent's scenarios, with why the others are off. */
export function criterionSources(
  agent: Pick<Agent, "modality" | "traceConnectionId"> | undefined,
): Record<CriterionSource, SourceAvailability> {
  if (!agent) {
    const off = { available: false, reason: "Pick an agent first." };
    return { transcript: { available: true }, trace: off, timing: off };
  }
  return {
    transcript: { available: true },
    trace: agent.traceConnectionId
      ? { available: true }
      : { available: false, reason: "The agent has no Langfuse connection to read traces from." },
    timing:
      agent.modality === "voice"
        ? { available: true }
        : { available: false, reason: "Timing only applies to voice agents." },
  };
}

/** Capability the simulator connection needs: an LLM for pipeline and text, realtime for realtime. */
export function simulatorCapability(modality: Modality, mode: SimulatorMode): Capability {
  return modality === "voice" && mode === "realtime" ? "realtime" : "llm";
}
