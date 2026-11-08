import type { SandboxChatReply, SandboxChatRequest } from "@/lib/api/types";

export const DEFAULT_USER_ID = "test-user";
export const MAX_MESSAGE_LENGTH = 8000;

export type ToolCallView = {
  name: string;
  /** Pretty-printed when the model sent valid JSON. */
  arguments: string;
  result: string;
  error: boolean;
  latency: string;
};

export type ReplyView = {
  message: string;
  toolCalls: ToolCallView[];
  usage: string;
};

export type NewChatEntry =
  | { role: "user"; text: string }
  | { role: "assistant"; reply: ReplyView }
  | { role: "error"; message: string };

/** One entry of the playground transcript. */
export type ChatEntry = NewChatEntry & { id: number };

/** Pretty-prints JSON arguments; anything else is shown as sent. */
export function formatToolArguments(raw: string): string {
  if (raw.trim() === "") return "{}";
  try {
    return JSON.stringify(JSON.parse(raw), null, 2);
  } catch {
    return raw;
  }
}

/** "850 ms", "1.2 s". */
export function formatLatency(ms: number): string {
  return ms < 1000 ? `${Math.round(ms)} ms` : `${(ms / 1000).toFixed(1)} s`;
}

export function formatUsage(usage: SandboxChatReply["usage"]): string {
  return `${usage.inputTokens.toLocaleString("en")} in · ${usage.outputTokens.toLocaleString("en")} out tokens`;
}

export function replyView(reply: SandboxChatReply): ReplyView {
  return {
    message: reply.message,
    toolCalls: reply.toolCalls.map((call) => ({
      name: call.name,
      arguments: formatToolArguments(call.arguments),
      result: call.result,
      error: call.error,
      latency: formatLatency(call.latencyMs),
    })),
    usage: formatUsage(reply.usage),
  };
}

/** Request body: a missing thread starts a new one; an empty user id falls back to the default. */
export function buildChatRequest(message: string, threadId: string | null, userId: string): SandboxChatRequest {
  const body: SandboxChatRequest = { message: message.trim(), userId: userId.trim() || DEFAULT_USER_ID };
  if (threadId) body.threadId = threadId;
  return body;
}
