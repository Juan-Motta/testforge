import { describe, expect, it } from "vitest";

import { criterionSources, distributeEvenly, simulatorCapability, weightTotal } from "./criteria";

describe("distributeEvenly", () => {
  it("splits 100 into integers that sum to exactly 100 for 1..30 criteria", () => {
    for (let count = 1; count <= 30; count++) {
      const weights = distributeEvenly(count);
      expect(weights).toHaveLength(count);
      expect(weights.reduce((sum, w) => sum + w, 0)).toBe(100);
      for (const w of weights) {
        expect(Number.isInteger(w)).toBe(true);
        expect(w).toBeGreaterThanOrEqual(1);
      }
      expect(Math.max(...weights) - Math.min(...weights)).toBeLessThanOrEqual(1);
    }
  });

  it("gives the remainder to the first criteria", () => {
    expect(distributeEvenly(3)).toEqual([34, 33, 33]);
    expect(distributeEvenly(1)).toEqual([100]);
    expect(distributeEvenly(0)).toEqual([]);
  });
});

describe("weightTotal", () => {
  it("is ready at exactly 100", () => {
    expect(weightTotal(["60", "40"])).toEqual({ total: 100, status: "ok", message: "Weights total 100." });
  });

  it("says how much is missing", () => {
    expect(weightTotal(["50", "30"])).toEqual({ total: 80, status: "under", message: "Add 20 to reach 100." });
  });

  it("says how much is over", () => {
    expect(weightTotal(["70", "45"])).toEqual({ total: 115, status: "over", message: "Remove 15 to get back to 100." });
  });

  it("counts blank or invalid weights as zero", () => {
    expect(weightTotal(["", "abc", "100"]).total).toBe(100);
    expect(weightTotal([]).status).toBe("under");
  });
});

describe("criterionSources", () => {
  it("allows every source for a voice agent with traces", () => {
    const sources = criterionSources({ modality: "voice", traceConnectionId: "c1" });
    expect(sources.transcript.available).toBe(true);
    expect(sources.trace.available).toBe(true);
    expect(sources.timing.available).toBe(true);
  });

  it("disables trace without a trace connection and timing for text agents, with a reason", () => {
    const sources = criterionSources({ modality: "text", traceConnectionId: null });
    expect(sources.transcript.available).toBe(true);
    expect(sources.trace).toEqual({ available: false, reason: "The agent has no Langfuse connection to read traces from." });
    expect(sources.timing).toEqual({ available: false, reason: "Timing only applies to voice agents." });
  });

  it("only allows the transcript until an agent is picked", () => {
    const sources = criterionSources(undefined);
    expect(sources.transcript.available).toBe(true);
    expect(sources.trace.available).toBe(false);
    expect(sources.timing.available).toBe(false);
    expect(sources.trace.reason).toBe("Pick an agent first.");
  });
});

describe("simulatorCapability", () => {
  it("needs an LLM for pipeline and text, realtime for realtime", () => {
    expect(simulatorCapability("voice", "pipeline")).toBe("llm");
    expect(simulatorCapability("voice", "realtime")).toBe("realtime");
    expect(simulatorCapability("text", "realtime")).toBe("llm");
  });
});
