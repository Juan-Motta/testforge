import type { Scenario } from "@/lib/api/types";

export const voiceScenario: Scenario = {
  id: "s1",
  name: "Refund a late fee",
  agentId: "agent-voice",
  personaId: "persona-1",
  groupId: "group-1",
  modality: "voice",
  tags: ["billing", "refunds"],
  context: "Customer since 2019.",
  goal: "Get the late fee refunded.",
  instructions: "Do not give the account number unless asked.",
  facts: [
    { key: "account", value: "88231" },
    { key: "zip", value: "06100" },
  ],
  criteria: [
    { id: "c1", text: "Verifies identity", kind: "must", source: "transcript", weight: 40 },
    { id: "c2", text: "Answers within 2s", kind: "should", source: "timing", weight: 60 },
  ],
  endConditions: { maxDurationSeconds: 300, maxSilenceSeconds: 10, maxTurns: 20, hangUpWhen: "goodbye" },
  interruptions: [{ agentTurn: 2, afterMs: 1500 }],
  simulator: { mode: "realtime", connectionId: "conn-rt", model: "gpt-realtime" },
  judge: { connectionId: "conn-llm", model: "gpt-5" },
  createdAt: "2026-11-07T10:00:00Z",
  updatedAt: "2026-11-07T10:00:00Z",
};
