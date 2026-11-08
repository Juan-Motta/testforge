import type {
  Capability,
  ConnectionKindId,
  SandboxAgent,
  SandboxAgentDraft,
  SandboxAgentInput,
  SandboxGenerateContext,
  SandboxMemorySettings,
  SandboxTool,
} from "@/lib/api/types";

export const MAX_TOOLS = 20;
export const TOOL_NAME_PATTERN = /^[a-zA-Z0-9_-]{1,64}$/;
export const RESERVED_TOOL_NAMES = ["remember", "recall"] as const;
export const DEFAULT_ERROR_MESSAGE = "The tool failed.";
export const MAX_LATENCY_MS = 30000;

/** Connection kinds a sandbox agent can run on. */
export const SANDBOX_CONNECTION_KINDS: readonly ConnectionKindId[] = ["openai", "azure_openai"];

/** Editor state of one mock tool. Numbers stay as typed text; `errorRate` is a percentage (0–100). */
export type ToolFormValues = {
  name: string;
  description: string;
  /** JSON Schema as text. */
  parameters: string;
  response: string;
  latencyMs: string;
  /** Percentage 0–100; the API stores a 0–1 fraction. */
  errorRate: string;
  errorMessage: string;
};

/** Editor state. `temperature` is "" when the provider default applies. */
export type SandboxFormValues = {
  name: string;
  description: string;
  modality: "text";
  connectionId: string;
  model: string;
  systemPrompt: string;
  temperature: string;
  memory: SandboxMemorySettings;
  tools: ToolFormValues[];
};

const EMPTY_PARAMETERS = { type: "object", properties: {}, required: [] };

export function formatJson(value: unknown): string {
  return JSON.stringify(value, null, 2);
}

export function emptyTool(): ToolFormValues {
  return {
    name: "",
    description: "",
    parameters: formatJson(EMPTY_PARAMETERS),
    response: "",
    latencyMs: "0",
    errorRate: "0",
    errorMessage: "",
  };
}

export function initialSandboxValues(): SandboxFormValues {
  return {
    name: "",
    description: "",
    modality: "text",
    connectionId: "",
    model: "",
    systemPrompt: "",
    temperature: "",
    memory: { conversation: true, longTerm: false },
    tools: [],
  };
}

/** 0–1 fraction → percentage text, e.g. 0.25 → "25". */
export function fractionToPercent(fraction: number): string {
  return String(Math.round(fraction * 10000) / 100);
}

/** Percentage text → 0–1 fraction, e.g. "12.5" → 0.125. */
export function percentToFraction(percent: string): number {
  return Math.round(Number(percent.trim()) * 100) / 10000;
}

export function toolFormValues(tool: SandboxTool): ToolFormValues {
  return {
    name: tool.name,
    description: tool.description,
    parameters: formatJson(tool.parameters),
    response: tool.response,
    latencyMs: String(tool.latencyMs),
    errorRate: fractionToPercent(tool.errorRate),
    errorMessage: tool.errorMessage,
  };
}

export function formValuesFromSandbox(agent: SandboxAgent): SandboxFormValues {
  return {
    name: agent.name,
    description: agent.description,
    modality: "text",
    connectionId: agent.connectionId,
    model: agent.model,
    systemPrompt: agent.systemPrompt,
    temperature: agent.temperature === null ? "" : String(agent.temperature),
    memory: { ...agent.memory },
    tools: agent.tools.map(toolFormValues),
  };
}

export type ParsedParameters =
  | { ok: true; value: Record<string, unknown> }
  | { ok: false; message: string };

/** Parses a tool's JSON Schema text; it must be a JSON object whose `type` is `object`. */
export function parseParameters(text: string): ParsedParameters {
  let value: unknown;
  try {
    value = JSON.parse(text);
  } catch {
    return { ok: false, message: "Enter valid JSON." };
  }
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    return { ok: false, message: "Enter a JSON Schema object." };
  }
  if ((value as { type?: unknown }).type !== "object") {
    return { ok: false, message: 'The schema needs "type": "object".' };
  }
  return { ok: true, value: value as Record<string, unknown> };
}

/** Client checks for one tool; the API validates again. Each returns `true` or a message. */
export const toolRules = {
  name(value: string, others: string[]): true | string {
    const name = value.trim();
    if (!TOOL_NAME_PATTERN.test(name)) return "Use 1–64 letters, digits, _ or -.";
    if ((RESERVED_TOOL_NAMES as readonly string[]).includes(name)) return `“${name}” is reserved for long-term memory.`;
    if (others.some((other) => other.trim() === name)) return "Another tool already has this name.";
    return true;
  },
  description(value: string): true | string {
    return value.trim() !== "" || "Describe when the model should call the tool.";
  },
  parameters(value: string): true | string {
    const parsed = parseParameters(value);
    return parsed.ok || parsed.message;
  },
  latencyMs(value: string): true | string {
    const number = Number(value.trim());
    if (value.trim() === "" || !Number.isInteger(number)) return "Enter a whole number of milliseconds.";
    if (number < 0 || number > MAX_LATENCY_MS) return `Use a value from 0 to ${MAX_LATENCY_MS}.`;
    return true;
  },
  errorRate(value: string): true | string {
    const number = Number(value.trim());
    if (value.trim() === "" || Number.isNaN(number)) return "Enter a percentage.";
    if (number < 0 || number > 100) return "Use a value from 0 to 100.";
    return true;
  },
};

export function temperatureRule(value: string): true | string {
  if (value.trim() === "") return true;
  const number = Number(value.trim());
  if (Number.isNaN(number) || number < 0 || number > 2) return "Use a value from 0 to 2.";
  return true;
}

export function buildTool(values: ToolFormValues): SandboxTool {
  const parsed = parseParameters(values.parameters);
  return {
    name: values.name.trim(),
    description: values.description.trim(),
    // Invalid JSON never reaches here past the client rules; send an empty object so the API reports it.
    parameters: parsed.ok ? parsed.value : {},
    response: values.response,
    latencyMs: Number(values.latencyMs.trim()),
    errorRate: percentToFraction(values.errorRate),
    errorMessage: values.errorMessage.trim(),
  };
}

export function buildSandboxInput(values: SandboxFormValues): SandboxAgentInput {
  const temperature = values.temperature.trim();
  return {
    name: values.name.trim(),
    description: values.description.trim(),
    modality: values.modality,
    connectionId: values.connectionId,
    model: values.model.trim(),
    systemPrompt: values.systemPrompt.trim(),
    temperature: temperature === "" ? null : Number(temperature),
    memory: { conversation: values.memory.conversation, longTerm: values.memory.longTerm },
    tools: values.tools.map(buildTool),
  };
}

/** Fills the form with an AI draft; the connection, model, temperature and modality stay. */
export function applyAgentDraft(values: SandboxFormValues, draft: SandboxAgentDraft): SandboxFormValues {
  return {
    ...values,
    name: draft.name,
    description: draft.description,
    systemPrompt: draft.systemPrompt,
    memory: { ...draft.memory },
    tools: draft.tools.slice(0, MAX_TOOLS).map(toolFormValues),
  };
}

/** The current draft, sent as context when generating one tool. */
export function generateContext(values: SandboxFormValues): SandboxGenerateContext {
  return {
    name: values.name.trim(),
    description: values.description.trim(),
    systemPrompt: values.systemPrompt.trim(),
    toolNames: values.tools.map((tool) => tool.name.trim()).filter(Boolean),
  };
}

/** Connections a sandbox agent can run on (OpenAI and Azure OpenAI). */
export function sandboxConnections<T extends { kind: ConnectionKindId }>(connections: T[]): T[] {
  return connections.filter((connection) => SANDBOX_CONNECTION_KINDS.includes(connection.kind));
}

/** Connections that can draft agents and tools (any LLM). */
export function llmConnections<T extends { capabilities: Capability[] }>(connections: T[]): T[] {
  return connections.filter((connection) => connection.capabilities.includes("llm"));
}

const TOP_FIELDS = new Set([
  "name",
  "description",
  "modality",
  "connectionId",
  "model",
  "systemPrompt",
  "temperature",
  "memory.conversation",
  "memory.longTerm",
]);
const TOOL_FIELDS = new Set(["name", "description", "parameters", "response", "latencyMs", "errorRate", "errorMessage"]);
const TOOL_PATH = /^tools\.(\d+)(?:\.(.+))?$/;

/**
 * Maps one API field key (`name`, `tools.0.parameters`, `tools.0.parameters.type`…) to a form path,
 * or null when no input shows it. Whole-list errors go to `tools.root`.
 */
export function sandboxFormPath(path: string, values: SandboxFormValues): string | null {
  if (TOP_FIELDS.has(path)) return path;
  if (path === "memory") return "memory.conversation";
  if (path === "tools") return "tools.root";
  const match = TOOL_PATH.exec(path);
  if (!match) return null;
  const index = Number(match[1]);
  if (index >= values.tools.length) return null;
  const field = (match[2] ?? "name").split(".")[0];
  return TOOL_FIELDS.has(field) ? `tools.${index}.${field}` : null;
}

/** Splits the API's `error.fields` into errors on a form path and messages with no input. */
export function mapSandboxFieldErrors(
  fields: Record<string, string>,
  values: SandboxFormValues,
): { matched: [string, string][]; unmatched: string[] } {
  const matched: [string, string][] = [];
  const unmatched: string[] = [];
  for (const [path, message] of Object.entries(fields)) {
    const formPath = sandboxFormPath(path, values);
    if (formPath) matched.push([formPath, message]);
    else unmatched.push(message);
  }
  return { matched, unmatched };
}
