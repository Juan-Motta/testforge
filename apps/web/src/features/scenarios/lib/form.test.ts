import { describe, expect, it } from "vitest";

import {
  buildScenarioInput,
  formValuesFromScenario,
  initialScenarioValues,
  mapScenarioFieldErrors,
  type ScenarioFormValues,
} from "./form";
import { voiceScenario } from "./test-fixtures";

describe("initialScenarioValues", () => {
  it("starts with one must criterion weighted 100 and the default end conditions", () => {
    expect(initialScenarioValues()).toEqual({
      name: "",
      agentId: "",
      personaId: "",
      groupId: "",
      tags: [],
      context: "",
      goal: "",
      instructions: "",
      facts: [],
      criteria: [{ criterionId: null, text: "", kind: "must", source: "transcript", weight: "100" }],
      endConditions: { maxDurationSeconds: "360", maxSilenceSeconds: "15", maxTurns: "20", hangUpWhen: "goal_or_goodbye" },
      interruptions: [],
      simulator: { mode: "pipeline", connectionId: "", model: "" },
      judge: { connectionId: "", model: "" },
    });
  });

  it("prefills the agent and group", () => {
    const values = initialScenarioValues({ agentId: "a1", groupId: "g1" });
    expect(values.agentId).toBe("a1");
    expect(values.groupId).toBe("g1");
  });
});

describe("buildScenarioInput", () => {
  it("round-trips a stored voice scenario, keeping criterion ids", () => {
    const input = buildScenarioInput(formValuesFromScenario(voiceScenario), "voice");
    const { agentId, personaId, groupId, name, tags, context, goal, instructions, facts, criteria } = voiceScenario;
    const { endConditions, interruptions, simulator, judge } = voiceScenario;
    expect(input).toEqual({
      name, agentId, personaId, groupId, tags, context, goal, instructions, facts, criteria,
      endConditions, interruptions, simulator, judge,
    });
  });

  it("sends null ids for new criteria and keeps existing ones", () => {
    const values = formValuesFromScenario(voiceScenario);
    values.criteria.push({ criterionId: null, text: " New one ", kind: "should", source: "transcript", weight: "10" });
    const input = buildScenarioInput(values, "voice");
    expect(input.criteria.map((c) => c.id)).toEqual(["c1", "c2", null]);
    expect(input.criteria[2]).toEqual({ id: null, text: "New one", kind: "should", source: "transcript", weight: 10 });
  });

  it("drops blank facts and interruptions, trims text and maps empty selects to null", () => {
    const values: ScenarioFormValues = {
      ...initialScenarioValues({ agentId: "a1" }),
      name: "  Name ",
      personaId: "p1",
      goal: " Goal ",
      facts: [
        { key: "", value: " " },
        { key: " plan ", value: "gold" },
      ],
      interruptions: [
        { agentTurn: "", afterMs: "" },
        { agentTurn: "3", afterMs: "800" },
      ],
    };
    const input = buildScenarioInput(values, "voice");
    expect(input.name).toBe("Name");
    expect(input.goal).toBe("Goal");
    expect(input.groupId).toBeNull();
    expect(input.facts).toEqual([{ key: "plan", value: "gold" }]);
    expect(input.interruptions).toEqual([{ agentTurn: 3, afterMs: 800 }]);
    expect(input.simulator).toEqual({ mode: "pipeline", connectionId: null, model: "" });
    expect(input.judge).toEqual({ connectionId: null, model: "" });
  });

  it("only sends voice settings for voice agents: interruptions, max silence and the voice mode", () => {
    const values = formValuesFromScenario(voiceScenario);
    values.endConditions.maxTurns = "abc";
    const input = buildScenarioInput(values, "voice");
    expect(input.interruptions).toHaveLength(1);
    expect(input.endConditions).toEqual({ maxDurationSeconds: 300, maxSilenceSeconds: 10, maxTurns: 20, hangUpWhen: "goodbye" });
    expect(input.simulator.mode).toBe("realtime");
  });

  it("only sends text settings for text agents: max turns, no interruptions and the text mode", () => {
    const values = formValuesFromScenario(voiceScenario);
    values.endConditions = { maxDurationSeconds: "120", maxSilenceSeconds: "", maxTurns: "8", hangUpWhen: "never" };
    const input = buildScenarioInput(values, "text");
    expect(input.interruptions).toEqual([]);
    expect(input.endConditions).toEqual({ maxDurationSeconds: 120, maxSilenceSeconds: 15, maxTurns: 8, hangUpWhen: "never" });
    expect(input.simulator.mode).toBe("text");
  });

  it("loads a text scenario with the pipeline mode in the form", () => {
    const values = formValuesFromScenario({ ...voiceScenario, modality: "text", simulator: { mode: "text", connectionId: null, model: "" } });
    expect(values.simulator).toEqual({ mode: "pipeline", connectionId: "", model: "" });
  });
});

describe("mapScenarioFieldErrors", () => {
  const values: ScenarioFormValues = {
    ...formValuesFromScenario(voiceScenario),
    facts: [
      { key: "", value: "" },
      { key: "account", value: "1" },
      { key: "zip", value: "2" },
    ],
    interruptions: [
      { agentTurn: "1", afterMs: "500" },
      { agentTurn: "", afterMs: "" },
      { agentTurn: "2", afterMs: "100" },
    ],
  };

  it("translates row indexes to form rows that include blank ones", () => {
    const { matched, unmatched } = mapScenarioFieldErrors(
      {
        "facts.0.key": "Duplicate key",
        "facts.1": "Bad fact",
        "interruptions.1.afterMs": "Too short",
        "criteria.1.weight": "Too high",
        "criteria.0.id": "Unknown id",
      },
      values,
      "voice",
    );
    expect(matched).toEqual([
      ["facts.1.key", "Duplicate key"],
      ["facts.2.key", "Bad fact"],
      ["interruptions.2.afterMs", "Too short"],
      ["criteria.1.weight", "Too high"],
      ["criteria.0.text", "Unknown id"],
    ]);
    expect(unmatched).toEqual([]);
  });

  it("maps scalar, list-level and aliased paths", () => {
    const { matched } = mapScenarioFieldErrors(
      {
        criteria: "Weights must total 100",
        "tags.3": "Too long",
        personaId: "Unknown persona",
        "endConditions.maxDurationSeconds": "Too long",
        "simulator.connectionId": "Needs llm",
        "judge.model": "Too long",
        simulator: "Bad",
        interruptions: "Codec does not support interruptions",
      },
      values,
      "voice",
    );
    expect(matched).toEqual([
      ["criteria", "Weights must total 100"],
      ["tags", "Too long"],
      ["personaId", "Unknown persona"],
      ["endConditions.maxDurationSeconds", "Too long"],
      ["simulator.connectionId", "Needs llm"],
      ["judge.model", "Too long"],
      ["simulator.mode", "Bad"],
      ["interruptions", "Codec does not support interruptions"],
    ]);
  });

  it("leaves paths without an input for the agent's modality unmatched", () => {
    const voice = mapScenarioFieldErrors({ "endConditions.maxTurns": "x", "facts.9.key": "y", "facts.0.nope": "z", id: "w" }, values, "voice");
    expect(voice.matched).toEqual([]);
    expect(voice.unmatched).toEqual(["x", "y", "z", "w"]);

    const text = mapScenarioFieldErrors(
      { "endConditions.maxSilenceSeconds": "a", "interruptions.0.afterMs": "b", interruptions: "c", "simulator.mode": "d", "endConditions.maxTurns": "e" },
      values,
      "text",
    );
    expect(text.matched).toEqual([["endConditions.maxTurns", "e"]]);
    expect(text.unmatched).toEqual(["a", "b", "c", "d"]);
  });
});
