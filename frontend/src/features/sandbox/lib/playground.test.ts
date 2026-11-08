import { describe, expect, it } from "vitest";

import type { SandboxChatReply } from "@/lib/api/types";

import { buildChatRequest, formatLatency, formatToolArguments, formatUsage, replyView } from "./playground";

const reply: SandboxChatReply = {
  message: "It is sunny in Lima.",
  threadId: "t-1",
  toolCalls: [
    { name: "get_weather", arguments: '{"city":"Lima"}', result: "It is sunny in Lima.", error: false, latencyMs: 320 },
    { name: "book", arguments: "not json", result: '{"error":"The tool failed."}', error: true, latencyMs: 1540 },
  ],
  usage: { inputTokens: 1234, outputTokens: 56 },
};

describe("playground reply", () => {
  it("renders tool calls with pretty arguments, result, error flag and latency, plus token usage", () => {
    expect(replyView(reply)).toEqual({
      message: "It is sunny in Lima.",
      toolCalls: [
        {
          name: "get_weather",
          arguments: '{\n  "city": "Lima"\n}',
          result: "It is sunny in Lima.",
          error: false,
          latency: "320 ms",
        },
        { name: "book", arguments: "not json", result: '{"error":"The tool failed."}', error: true, latency: "1.5 s" },
      ],
      usage: "1,234 in · 56 out tokens",
    });
  });

  it("formats empty arguments, latency and usage", () => {
    expect(formatToolArguments("")).toBe("{}");
    expect(formatLatency(0)).toBe("0 ms");
    expect(formatLatency(999)).toBe("999 ms");
    expect(formatLatency(1000)).toBe("1.0 s");
    expect(formatUsage({ inputTokens: 0, outputTokens: 0 })).toBe("0 in · 0 out tokens");
  });
});

describe("chat request", () => {
  it("omits the thread to start a new one and keeps it afterwards", () => {
    expect(buildChatRequest(" Hi ", null, "test-user")).toEqual({ message: "Hi", userId: "test-user" });
    expect(buildChatRequest("Hi", "t-1", "ana")).toEqual({ message: "Hi", threadId: "t-1", userId: "ana" });
  });

  it("falls back to the default user id when it is empty", () => {
    expect(buildChatRequest("Hi", null, "  ")).toEqual({ message: "Hi", userId: "test-user" });
  });
});
