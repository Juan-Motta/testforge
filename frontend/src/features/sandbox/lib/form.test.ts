import { describe, expect, it } from "vitest";

import type { Capability, ConnectionKindId, SandboxAgent, SandboxAgentDraft } from "@/lib/api/types";

import {
  applyAgentDraft,
  buildSandboxInput,
  emptyTool,
  formValuesFromSandbox,
  fractionToPercent,
  generateContext,
  initialSandboxValues,
  llmConnections,
  mapSandboxFieldErrors,
  parseParameters,
  percentToFraction,
  sandboxConnections,
  temperatureRule,
  toolRules,
  type SandboxFormValues,
} from "./form";

const weatherTool = {
  name: "get_weather",
  description: "Current weather for a city.",
  parameters: { type: "object", properties: { city: { type: "string" } }, required: ["city"] },
  response: "It is sunny in {{args.city}}.",
  latencyMs: 300,
  errorRate: 0.25,
  errorMessage: "Weather service down.",
};

const agent: SandboxAgent = {
  id: "s1",
  name: "Weather bot",
  description: "Answers weather questions.",
  modality: "text",
  connectionId: "c-openai",
  model: "gpt-5-mini",
  systemPrompt: "You are a weather assistant.",
  temperature: 0.7,
  memory: { conversation: true, longTerm: true },
  tools: [weatherTool],
  createdAt: "2026-11-07T10:00:00Z",
  updatedAt: "2026-11-07T10:00:00Z",
};

describe("error rate", () => {
  it("shows a 0–1 fraction as a percentage and stores it back as a fraction", () => {
    expect(fractionToPercent(0.25)).toBe("25");
    expect(fractionToPercent(0.125)).toBe("12.5");
    expect(fractionToPercent(0)).toBe("0");
    expect(fractionToPercent(1)).toBe("100");
    expect(percentToFraction("25")).toBe(0.25);
    expect(percentToFraction(" 12.5 ")).toBe(0.125);
    expect(percentToFraction("100")).toBe(1);
    expect(percentToFraction("33.33")).toBe(0.3333);
  });

  it("accepts 0 to 100 only", () => {
    expect(toolRules.errorRate("0")).toBe(true);
    expect(toolRules.errorRate("100")).toBe(true);
    expect(toolRules.errorRate("101")).toBe("Use a value from 0 to 100.");
    expect(toolRules.errorRate("-1")).toBe("Use a value from 0 to 100.");
    expect(toolRules.errorRate("")).toBe("Enter a percentage.");
  });
});

describe("parameters JSON Schema", () => {
  it("accepts a JSON object whose type is object", () => {
    expect(parseParameters('{"type":"object","properties":{}}')).toEqual({
      ok: true,
      value: { type: "object", properties: {} },
    });
  });

  it("rejects invalid JSON, non-objects and other schema types", () => {
    expect(parseParameters("{type: object}")).toEqual({ ok: false, message: "Enter valid JSON." });
    expect(parseParameters("[]")).toEqual({ ok: false, message: "Enter a JSON Schema object." });
    expect(parseParameters("null")).toEqual({ ok: false, message: "Enter a JSON Schema object." });
    expect(parseParameters('{"type":"string"}')).toEqual({ ok: false, message: 'The schema needs "type": "object".' });
    expect(toolRules.parameters('{"properties":{}}')).toBe('The schema needs "type": "object".');
  });

  it("starts new tools with a valid empty schema", () => {
    expect(toolRules.parameters(emptyTool().parameters)).toBe(true);
  });
});

describe("tool rules", () => {
  it("checks the name pattern, reserved names and duplicates", () => {
    expect(toolRules.name("get_weather", [])).toBe(true);
    expect(toolRules.name("get weather", [])).toBe("Use 1–64 letters, digits, _ or -.");
    expect(toolRules.name("a".repeat(65), [])).toBe("Use 1–64 letters, digits, _ or -.");
    expect(toolRules.name("remember", [])).toBe("“remember” is reserved for long-term memory.");
    expect(toolRules.name("recall", [])).toBe("“recall” is reserved for long-term memory.");
    expect(toolRules.name("lookup", ["lookup"])).toBe("Another tool already has this name.");
  });

  it("checks latency and temperature ranges", () => {
    expect(toolRules.latencyMs("30000")).toBe(true);
    expect(toolRules.latencyMs("30001")).toBe("Use a value from 0 to 30000.");
    expect(toolRules.latencyMs("1.5")).toBe("Enter a whole number of milliseconds.");
    expect(temperatureRule("")).toBe(true);
    expect(temperatureRule("2")).toBe(true);
    expect(temperatureRule("2.1")).toBe("Use a value from 0 to 2.");
  });
});

describe("form → SandboxAgentInput", () => {
  it("starts a text agent with conversation memory and no tools", () => {
    expect(initialSandboxValues()).toEqual({
      name: "",
      description: "",
      modality: "text",
      connectionId: "",
      model: "",
      systemPrompt: "",
      temperature: "",
      memory: { conversation: true, longTerm: false },
      tools: [],
    });
  });

  it("round-trips a stored agent", () => {
    const values = formValuesFromSandbox(agent);
    expect(values.temperature).toBe("0.7");
    expect(values.tools[0].errorRate).toBe("25");
    expect(values.tools[0].latencyMs).toBe("300");
    expect(JSON.parse(values.tools[0].parameters)).toEqual(weatherTool.parameters);
    expect(buildSandboxInput(values)).toEqual({
      name: "Weather bot",
      description: "Answers weather questions.",
      modality: "text",
      connectionId: "c-openai",
      model: "gpt-5-mini",
      systemPrompt: "You are a weather assistant.",
      temperature: 0.7,
      memory: { conversation: true, longTerm: true },
      tools: [weatherTool],
    });
  });

  it("trims text, sends a null temperature when empty and converts tool numbers", () => {
    const values: SandboxFormValues = {
      ...initialSandboxValues(),
      name: "  Bot ",
      connectionId: "c",
      model: " gpt-5 ",
      systemPrompt: " Be brief. ",
      temperature: "  ",
      tools: [
        {
          ...emptyTool(),
          name: " lookup ",
          description: " Looks things up. ",
          parameters: '{"type":"object"}',
          response: " {{args.q}} ",
          latencyMs: "1500",
          errorRate: "10",
          errorMessage: "  ",
        },
      ],
    };
    const input = buildSandboxInput(values);
    expect(input).toMatchObject({ name: "Bot", model: "gpt-5", systemPrompt: "Be brief.", temperature: null });
    expect(input.tools).toEqual([
      {
        name: "lookup",
        description: "Looks things up.",
        parameters: { type: "object" },
        // The mock response is sent exactly as typed.
        response: " {{args.q}} ",
        latencyMs: 1500,
        errorRate: 0.1,
        errorMessage: "",
      },
    ]);
  });
});

describe("server field errors", () => {
  const values = formValuesFromSandbox(agent);

  it("maps top-level and tool fields to form paths", () => {
    expect(
      mapSandboxFieldErrors(
        {
          name: "Name is taken.",
          connectionId: "Sandbox agents support OpenAI and Azure OpenAI connections.",
          modality: "Voice sandbox agents are not available yet.",
          "tools.0.name": "Duplicate tool name.",
          "tools.0.parameters": "Its type must be object.",
          "tools.0.errorRate": "Use 0 to 1.",
          "memory.longTerm": "Invalid.",
        },
        values,
      ),
    ).toEqual({
      matched: [
        ["name", "Name is taken."],
        ["connectionId", "Sandbox agents support OpenAI and Azure OpenAI connections."],
        ["modality", "Voice sandbox agents are not available yet."],
        ["tools.0.name", "Duplicate tool name."],
        ["tools.0.parameters", "Its type must be object."],
        ["tools.0.errorRate", "Use 0 to 1."],
        ["memory.longTerm", "Invalid."],
      ],
      unmatched: [],
    });
  });

  it("puts list errors on tools.root, nested schema keys on the parameters input and unknown keys aside", () => {
    expect(
      mapSandboxFieldErrors(
        {
          tools: "At most 20 tools.",
          "tools.0.parameters.type": "Must be object.",
          "tools.0": "Invalid tool.",
          "tools.3.name": "Out of range.",
          other: "Something else.",
        },
        values,
      ),
    ).toEqual({
      matched: [
        ["tools.root", "At most 20 tools."],
        ["tools.0.parameters", "Must be object."],
        ["tools.0.name", "Invalid tool."],
      ],
      unmatched: ["Out of range.", "Something else."],
    });
  });
});

describe("generated drafts", () => {
  const draft: SandboxAgentDraft = {
    name: "Pizza bot",
    description: "Takes pizza orders.",
    systemPrompt: "You take pizza orders.",
    memory: { conversation: true, longTerm: false },
    tools: [{ ...weatherTool, name: "place_order", errorRate: 0.05 }],
  };

  it("overwrites identity, prompt, memory and tools and keeps the connection, model and temperature", () => {
    const current = formValuesFromSandbox(agent);
    const next = applyAgentDraft(current, draft);
    expect(next).toMatchObject({
      name: "Pizza bot",
      description: "Takes pizza orders.",
      systemPrompt: "You take pizza orders.",
      memory: { conversation: true, longTerm: false },
      connectionId: "c-openai",
      model: "gpt-5-mini",
      temperature: "0.7",
      modality: "text",
    });
    expect(next.tools).toHaveLength(1);
    expect(next.tools[0]).toMatchObject({ name: "place_order", errorRate: "5", latencyMs: "300" });
    // The form is not mutated.
    expect(current.name).toBe("Weather bot");
  });

  it("sends the current draft as context for a tool", () => {
    const values = { ...formValuesFromSandbox(agent), tools: [...formValuesFromSandbox(agent).tools, emptyTool()] };
    expect(generateContext(values)).toEqual({
      name: "Weather bot",
      description: "Answers weather questions.",
      systemPrompt: "You are a weather assistant.",
      toolNames: ["get_weather"],
    });
  });
});

describe("connection filters", () => {
  const connections: { id: string; kind: ConnectionKindId; capabilities: Capability[] }[] = [
    { id: "a", kind: "openai", capabilities: ["llm", "tts"] },
    { id: "b", kind: "azure_openai", capabilities: ["llm"] },
    { id: "c", kind: "anthropic", capabilities: ["llm"] },
    { id: "d", kind: "azure_speech", capabilities: ["tts", "stt"] },
  ];

  it("runs agents only on OpenAI and Azure OpenAI", () => {
    expect(sandboxConnections(connections).map((c) => c.id)).toEqual(["a", "b"]);
  });

  it("generates with any LLM connection", () => {
    expect(llmConnections(connections).map((c) => c.id)).toEqual(["a", "b", "c"]);
  });
});
