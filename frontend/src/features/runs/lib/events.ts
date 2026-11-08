import type { RunMetrics, RunStatus, TranscriptTurn } from "@/lib/api/types";

import { isRunStatus } from "./status";

export const RUN_EVENT_TYPES = ["status", "transcript", "metrics", "log", "end"] as const;
export type RunEventType = (typeof RUN_EVENT_TYPES)[number];

/** One Server-Sent Event of `GET .../runs/{id}/events`, with its parsed JSON data. */
export type RunEvent = { id: number | null; type: RunEventType; data: unknown };

export type LiveTurn = TranscriptTurn & { index: number; final: boolean };
export type LogLevel = "info" | "warn" | "error";
export type LiveLog = { id: number | null; level: LogLevel; message: string };

export type LiveRunState = {
  /** Highest event id applied; replays at or below it are ignored. */
  lastEventId: number;
  status: RunStatus | null;
  endReason: string | null;
  error: string | null;
  /** Ordered by index. */
  turns: LiveTurn[];
  metrics: RunMetrics | null;
  logs: LiveLog[];
  ended: boolean;
};

export const INITIAL_LIVE_RUN: LiveRunState = {
  lastEventId: 0,
  status: null,
  endReason: null,
  error: null,
  turns: [],
  metrics: null,
  logs: [],
  ended: false,
};

export function parseRunEvent(type: string, data: string, lastEventId: string): RunEvent | null {
  if (!(RUN_EVENT_TYPES as readonly string[]).includes(type)) return null;
  let parsed: unknown;
  try {
    parsed = JSON.parse(data);
  } catch {
    return null;
  }
  const id = lastEventId === "" ? null : Number(lastEventId);
  return { id: id === null || Number.isFinite(id) ? id : null, type: type as RunEventType, data: parsed };
}

type Obj = Record<string, unknown>;
const isObject = (value: unknown): value is Obj => typeof value === "object" && value !== null;
const stringOrNull = (value: unknown) => (typeof value === "string" ? value : null);

function isTurn(value: unknown): value is TranscriptTurn {
  return (
    isObject(value) &&
    (value.role === "user" || value.role === "agent") &&
    typeof value.text === "string" &&
    typeof value.startMs === "number"
  );
}

function upsertTurn(turns: LiveTurn[], next: LiveTurn): LiveTurn[] {
  const position = turns.findIndex((turn) => turn.index >= next.index);
  if (position === -1) return [...turns, next];
  if (turns[position].index === next.index) return turns.map((turn, i) => (i === position ? next : turn));
  return [...turns.slice(0, position), next, ...turns.slice(position)];
}

function applyData(state: LiveRunState, event: RunEvent): LiveRunState {
  const data = event.data;
  if (!isObject(data)) return state;
  switch (event.type) {
    case "status":
      if (!isRunStatus(data.status)) return state;
      return { ...state, status: data.status, endReason: stringOrNull(data.endReason), error: stringOrNull(data.error) };
    case "transcript": {
      if (typeof data.index !== "number" || !isTurn(data.turn)) return state;
      const turn: LiveTurn = {
        ...data.turn,
        latencyMs: data.turn.latencyMs ?? null,
        interrupted: data.turn.interrupted === true,
        endMs: data.turn.endMs ?? data.turn.startMs,
        index: data.index,
        final: data.final === true,
      };
      return { ...state, turns: upsertTurn(state.turns, turn) };
    }
    case "metrics":
      if (typeof data.durationMs !== "number") return state;
      return { ...state, metrics: data as RunMetrics };
    case "log": {
      if (typeof data.message !== "string") return state;
      const level: LogLevel = data.level === "warn" || data.level === "error" ? data.level : "info";
      return { ...state, logs: [...state.logs, { id: event.id, level, message: data.message }] };
    }
    case "end":
      return { ...state, ended: true, status: isRunStatus(data.status) ? data.status : state.status };
  }
}

/** Applies one event; replayed events (id at or below the last applied one) leave the state unchanged. */
export function applyRunEvent(state: LiveRunState, event: RunEvent): LiveRunState {
  if (event.id !== null && event.id <= state.lastEventId) return state;
  const next = applyData(state, event);
  if (event.id === null) return next;
  return { ...next, lastEventId: event.id };
}

/** The agent is still speaking: the last turn is an agent turn that is not final. */
export function speakingTurn(turns: readonly LiveTurn[]): boolean {
  const last = turns.at(-1);
  return last !== undefined && last.role === "agent" && !last.final;
}
