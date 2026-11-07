import type { Capability } from "@/lib/api/types";

export const CAPABILITY_LABELS: Record<Capability, string> = {
  llm: "LLM",
  tts: "Text to speech",
  stt: "Speech to text",
  realtime: "Realtime",
  traces: "Traces",
};
