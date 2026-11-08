import { describe, expect, it } from "vitest";

import type { RunMetrics, TranscriptTurn } from "@/lib/api/types";

import { applyRunEvent, INITIAL_LIVE_RUN, parseRunEvent, speakingTurn, type RunEvent } from "./events";

const turn = (role: TranscriptTurn["role"], text: string, startMs = 0): TranscriptTurn => ({
  role,
  text,
  startMs,
  endMs: startMs + 1000,
  latencyMs: role === "agent" ? 900 : null,
  interrupted: false,
});

const metrics: RunMetrics = {
  durationMs: 12_000,
  userTurns: 2,
  agentTurns: 2,
  latencyP50Ms: 800,
  latencyP95Ms: 1200,
  agentAudioMs: 6000,
  llmInputTokens: 300,
  llmOutputTokens: 40,
};

function reduce(events: RunEvent[], state = INITIAL_LIVE_RUN) {
  return events.reduce(applyRunEvent, state);
}

describe("parseRunEvent", () => {
  it("parses the id and JSON data", () => {
    expect(parseRunEvent("status", '{"status":"running","endReason":null,"error":null}', "7")).toEqual({
      id: 7,
      type: "status",
      data: { status: "running", endReason: null, error: null },
    });
  });

  it("returns null for malformed data or unknown events", () => {
    expect(parseRunEvent("status", "{nope", "1")).toBeNull();
    expect(parseRunEvent("weird", "{}", "1")).toBeNull();
  });

  it("accepts events without an id", () => {
    expect(parseRunEvent("end", '{"status":"completed"}', "")?.id).toBeNull();
  });
});

describe("applyRunEvent", () => {
  it("upserts transcript turns by index while they grow", () => {
    const state = reduce([
      { id: 1, type: "transcript", data: { index: 0, turn: turn("user", "Hi"), final: true } },
      { id: 2, type: "transcript", data: { index: 1, turn: turn("agent", "Hel"), final: false } },
      { id: 3, type: "transcript", data: { index: 1, turn: turn("agent", "Hello, how can I help?"), final: true } },
    ]);
    expect(state.turns.map((t) => [t.index, t.text, t.final])).toEqual([
      [0, "Hi", true],
      [1, "Hello, how can I help?", true],
    ]);
  });

  it("keeps turns ordered by index when they arrive out of order", () => {
    const state = reduce([
      { id: 1, type: "transcript", data: { index: 1, turn: turn("agent", "B"), final: true } },
      { id: 2, type: "transcript", data: { index: 0, turn: turn("user", "A"), final: true } },
    ]);
    expect(state.turns.map((t) => t.text)).toEqual(["A", "B"]);
  });

  it("tracks status, metrics, logs and the end", () => {
    const state = reduce([
      { id: 1, type: "status", data: { status: "running", endReason: null, error: null } },
      { id: 2, type: "log", data: { level: "info", message: "connected" } },
      { id: 3, type: "metrics", data: metrics },
      { id: 4, type: "status", data: { status: "failed", endReason: "error", error: "agent closed" } },
      { id: 5, type: "end", data: { status: "failed" } },
    ]);
    expect(state.status).toBe("failed");
    expect(state.endReason).toBe("error");
    expect(state.error).toBe("agent closed");
    expect(state.metrics).toEqual(metrics);
    expect(state.logs).toEqual([{ id: 2, level: "info", message: "connected" }]);
    expect(state.ended).toBe(true);
    expect(state.lastEventId).toBe(5);
  });

  it("ignores replayed events so a reconnect does not duplicate logs", () => {
    const events: RunEvent[] = [
      { id: 1, type: "log", data: { level: "info", message: "a" } },
      { id: 2, type: "transcript", data: { index: 0, turn: turn("user", "Hi"), final: true } },
    ];
    const once = reduce(events);
    const twice = reduce(events, once);
    expect(twice).toBe(once);
    expect(twice.logs).toHaveLength(1);
  });

  it("ignores payloads with the wrong shape", () => {
    const state = reduce([
      { id: 1, type: "transcript", data: { index: "x" } },
      { id: 2, type: "status", data: { status: "bogus" } },
      { id: 3, type: "log", data: null },
    ]);
    expect(state.turns).toEqual([]);
    expect(state.status).toBeNull();
    expect(state.logs).toEqual([]);
  });
});

describe("speakingTurn", () => {
  it("is true when the last turn is an agent turn still growing", () => {
    const growing = reduce([{ id: 1, type: "transcript", data: { index: 0, turn: turn("agent", "Hel"), final: false } }]);
    expect(speakingTurn(growing.turns)).toBe(true);
    const done = reduce([{ id: 2, type: "transcript", data: { index: 0, turn: turn("agent", "Hello"), final: true } }], growing);
    expect(speakingTurn(done.turns)).toBe(false);
    expect(speakingTurn([])).toBe(false);
  });
});
