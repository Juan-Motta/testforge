import type { Agent, AgentCodecSpec, AgentPreset } from "@/lib/api/types";

export const viviLiveCodec: AgentCodecSpec = {
  codec: "vivi_live",
  label: "Vivi Live",
  description: "Vivi voice events over WebSocket.",
  modality: "voice",
  fields: [{ key: "sampleRate", label: "Sample rate", type: "number", required: false, default: "24000" }],
};

export const jsonAudioCodec: AgentCodecSpec = {
  codec: "json_audio",
  label: "Generic JSON audio",
  description: "Configurable JSON events with base64 audio.",
  modality: "voice",
  fields: [
    { key: "audioPath", label: "Agent audio path", type: "text", required: true, placeholder: "audio" },
    { key: "sendTemplate", label: "Send audio template", type: "json", required: true },
  ],
};

export const httpStreamCodec: AgentCodecSpec = {
  codec: "http_stream",
  label: "HTTP stream",
  description: "Line-delimited data: JSON.",
  modality: "text",
  fields: [{ key: "deltaPath", label: "Delta path", type: "text", required: false, default: "data.chunk" }],
};

export const codecs: AgentCodecSpec[] = [viviLiveCodec, jsonAudioCodec, httpStreamCodec];

export const viviTextPreset: AgentPreset = {
  id: "vivi_text",
  label: "Vivi text",
  description: "Vivi channels API over HTTP streaming.",
  modality: "text",
  draft: {
    transport: {
      url: "https://api.vivi.example/channelsApi/{{var.channelId}}/stream",
      query: [],
      headers: [{ key: "Authorization", value: "Bearer {{secret.apiKey}}" }],
      body: '{"message": "{{message}}", "threadId": "{{run.thread_id}}"}',
      initMessages: [],
      preSession: null,
    },
    codec: { type: "http_stream", options: { deltaPath: "data.chunk" } },
    variables: [{ key: "channelId", value: "" }],
    traceSessionId: "{{run.thread_id}}",
  },
  secretNames: ["apiKey"],
};

export const relayPreset: AgentPreset = {
  id: "relay",
  label: "Generic JSON WebSocket",
  description: "Any JSON WebSocket agent.",
  modality: "voice",
  draft: {
    transport: {
      url: "wss://relay.example/ws/{{session.sessionId}}",
      query: [{ key: "token", value: "{{secret.token}}" }],
      headers: [],
      body: null,
      initMessages: ['{"type": "start"}'],
      preSession: {
        method: "POST",
        url: "https://relay.example/sessions",
        headers: [],
        body: "{}",
        captures: [{ name: "sessionId", path: "session_id" }],
      },
    },
    codec: { type: "json_audio", options: { audioPath: "audio" } },
    variables: [],
    traceSessionId: "{{session.sessionId}}",
  },
  secretNames: ["token"],
};

export const agent: Agent = {
  id: "5a0c4a9e-2f1d-4d8e-9a51-1d2b3c4d5e6f",
  name: "Support bot",
  description: "Answers billing questions.",
  modality: "voice",
  transport: {
    url: "wss://agent.example/ws",
    query: [{ key: "lang", value: "en" }],
    headers: [{ key: "Authorization", value: "Bearer {{secret.apiKey}}" }],
    body: null,
    initMessages: ['{"type": "hello"}'],
    preSession: null,
  },
  codec: { type: "vivi_live", options: { sampleRate: "16000" } },
  variables: [{ key: "tenant", value: "acme" }],
  secrets: [{ name: "apiKey", hint: "•••• 9f2c" }],
  traceConnectionId: "7b0c4a9e-2f1d-4d8e-9a51-1d2b3c4d5e6f",
  traceSessionId: "{{run.thread_id}}",
  createdBy: { id: "1b0c4a9e-2f1d-4d8e-9a51-1d2b3c4d5e6f", name: "Ada" },
  createdAt: "2026-11-01T10:00:00Z",
  updatedAt: "2026-11-01T10:00:00Z",
};
