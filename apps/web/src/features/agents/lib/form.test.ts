import { describe, expect, it } from "vitest";

import {
  applyPreset,
  buildAgentInput,
  formValuesFromAgent,
  initialFormValues,
  mapServerFieldErrors,
  templateVariables,
  type AgentFormValues,
} from "./form";
import { agent, codecs, relayPreset, viviTextPreset } from "./test-fixtures";

describe("initialFormValues", () => {
  it("starts an empty voice agent with the default trace session id", () => {
    expect(initialFormValues()).toEqual({
      name: "",
      description: "",
      modality: "voice",
      transport: {
        url: "",
        query: [],
        headers: [],
        body: "",
        initMessages: [],
        preSessionEnabled: false,
        preSession: { method: "POST", url: "", headers: [], body: "", captures: [] },
      },
      codec: { type: "", options: {} },
      variables: [],
      secrets: [],
      traceConnectionId: "",
      traceSessionId: "{{run.thread_id}}",
    });
  });
});

describe("formValuesFromAgent", () => {
  it("loads the stored definition with empty secret values and their hints", () => {
    const values = formValuesFromAgent(agent);
    expect(values.secrets).toEqual([{ name: "apiKey", value: "", hint: "•••• 9f2c" }]);
    expect(values.transport.initMessages).toEqual([{ value: '{"type": "hello"}' }]);
    expect(values.transport.preSessionEnabled).toBe(false);
    expect(values.codec).toEqual({ type: "vivi_live", options: { sampleRate: "16000" } });
    expect(values.traceConnectionId).toBe(agent.traceConnectionId);
  });

  it("lists every option of the codec, empty when unset, so the form starts clean", () => {
    const values = formValuesFromAgent({ ...agent, codec: { type: "json_audio", options: { audioPath: "audio" } } }, codecs);
    expect(values.codec.options).toEqual({ audioPath: "audio", sendTemplate: "" });
    expect(buildAgentInput(values).codec).toEqual({ type: "json_audio", options: { audioPath: "audio" } });
  });

  it("round-trips through buildAgentInput, keeping stored secrets with an empty value", () => {
    expect(buildAgentInput(formValuesFromAgent(agent))).toEqual({
      name: "Support bot",
      description: "Answers billing questions.",
      modality: "voice",
      transport: agent.transport,
      codec: agent.codec,
      variables: agent.variables,
      secrets: [{ name: "apiKey", value: "" }],
      traceConnectionId: agent.traceConnectionId,
      traceSessionId: "{{run.thread_id}}",
    });
  });
});

function textValues(): AgentFormValues {
  const values = initialFormValues("text");
  return {
    ...values,
    name: "  Chat bot ",
    transport: {
      ...values.transport,
      url: " https://chat.example/api ",
      query: [
        { key: "", value: "" },
        { key: "v", value: "2" },
      ],
      headers: [
        { key: "X-Key", value: "{{secret.key}}" },
        { key: "  ", value: " " },
      ],
      body: '{"text": "{{message}}"}',
      initMessages: [{ value: '{"ignored": true}' }],
      preSession: { method: "GET", url: "https://ignored", headers: [], body: "x", captures: [] },
    },
    codec: { type: "http_json", options: { replyPath: " reply ", empty: "  " } },
    variables: [
      { key: "tenant", value: "" },
      { key: "", value: "" },
    ],
    secrets: [
      { name: "key", value: "" },
      { name: "", value: "" },
    ],
    traceConnectionId: "",
    traceSessionId: "  ",
  };
}

describe("buildAgentInput", () => {
  it("trims, drops blank rows, keeps empty values and sends the body only for text agents", () => {
    expect(buildAgentInput(textValues())).toEqual({
      name: "Chat bot",
      description: "",
      modality: "text",
      transport: {
        url: "https://chat.example/api",
        query: [{ key: "v", value: "2" }],
        headers: [{ key: "X-Key", value: "{{secret.key}}" }],
        body: '{"text": "{{message}}"}',
        initMessages: [],
        preSession: null,
      },
      codec: { type: "http_json", options: { replyPath: "reply" } },
      variables: [{ key: "tenant", value: "" }],
      secrets: [{ name: "key", value: "" }],
      traceConnectionId: null,
      traceSessionId: "{{run.thread_id}}",
    });
  });

  it("sends init messages only for voice agents and never a body", () => {
    const values = initialFormValues("voice");
    values.transport.body = '{"message": "{{message}}"}';
    values.transport.initMessages = [{ value: '{"type": "start"}' }, { value: "  " }];
    const input = buildAgentInput(values);
    expect(input.transport.body).toBeNull();
    expect(input.transport.initMessages).toEqual(['{"type": "start"}']);
  });

  it("includes the pre-session call when enabled, with a body only for POST", () => {
    const values = initialFormValues("voice");
    values.transport.preSessionEnabled = true;
    values.transport.preSession = {
      method: "GET",
      url: " https://relay.example/sessions ",
      headers: [{ key: "", value: "" }],
      body: "{}",
      captures: [
        { name: "sessionId", path: " session_id " },
        { name: "", path: "" },
      ],
    };
    expect(buildAgentInput(values).transport.preSession).toEqual({
      method: "GET",
      url: "https://relay.example/sessions",
      headers: [],
      body: null,
      captures: [{ name: "sessionId", path: "session_id" }],
    });
    values.transport.preSession.method = "POST";
    expect(buildAgentInput(values).transport.preSession?.body).toBe("{}");
  });

  it("leaves out codec options the form registered without a value", () => {
    const values = initialFormValues();
    values.codec = { type: "json_audio", options: { audioPath: "audio", sendTemplate: undefined as unknown as string } };
    expect(buildAgentInput(values).codec).toEqual({ type: "json_audio", options: { audioPath: "audio" } });
  });

  it("sends a new secret value trimmed", () => {
    const values = initialFormValues();
    values.secrets = [{ name: " token ", value: " s3cret ", hint: "•••• 1234" }];
    expect(buildAgentInput(values).secrets).toEqual([{ name: "token", value: "s3cret" }]);
  });
});

describe("applyPreset", () => {
  it("fills transport, codec, variables, modality and empty secret rows", () => {
    const values = applyPreset(initialFormValues(), viviTextPreset);
    expect(values.name).toBe("Vivi text");
    expect(values.modality).toBe("text");
    expect(values.transport).toEqual({
      url: viviTextPreset.draft.transport.url,
      query: [],
      headers: [{ key: "Authorization", value: "Bearer {{secret.apiKey}}" }],
      body: viviTextPreset.draft.transport.body,
      initMessages: [],
      preSessionEnabled: false,
      preSession: { method: "POST", url: "", headers: [], body: "", captures: [] },
    });
    expect(values.codec).toEqual({ type: "http_stream", options: { deltaPath: "data.chunk" } });
    expect(values.variables).toEqual([{ key: "channelId", value: "" }]);
    expect(values.secrets).toEqual([{ name: "apiKey", value: "" }]);
    expect(values.traceSessionId).toBe("{{run.thread_id}}");
  });

  it("keeps the name, description and existing secrets, adding only missing names", () => {
    const current = formValuesFromAgent(agent);
    current.secrets.push({ name: "token", value: "typed" });
    const values = applyPreset(current, relayPreset);
    expect(values.name).toBe("Support bot");
    expect(values.description).toBe("Answers billing questions.");
    expect(values.secrets).toEqual([
      { name: "apiKey", value: "", hint: "•••• 9f2c" },
      { name: "token", value: "typed" },
    ]);
    expect(values.transport.preSessionEnabled).toBe(true);
    expect(values.transport.preSession.captures).toEqual([{ name: "sessionId", path: "session_id" }]);
    expect(values.transport.initMessages).toEqual([{ value: '{"type": "start"}' }]);
    expect(values.traceSessionId).toBe("{{session.sessionId}}");
  });

  it("does not share arrays with the preset", () => {
    const values = applyPreset(initialFormValues(), viviTextPreset);
    values.transport.headers[0].value = "changed";
    expect(viviTextPreset.draft.transport.headers[0].value).toBe("Bearer {{secret.apiKey}}");
  });
});

describe("mapServerFieldErrors", () => {
  it("maps dotted API paths onto form fields, accounting for dropped blank rows", () => {
    const values = textValues();
    values.codec.type = "http_stream";
    const { matched, unmatched } = mapServerFieldErrors(
      {
        name: "Name is taken.",
        "transport.url": "Use https.",
        "transport.query.0.key": "Bad key.",
        "transport.headers.0.value": "Unknown secret.",
        "transport.body": "Must reference {{message}}.",
        "codec.options.deltaPath": "Invalid path.",
        "variables.0.key": "Duplicate.",
        secrets: "Enter every secret again.",
        "secrets.0.value": "Too long.",
        traceConnectionId: "Not a traces connection.",
        "codec.options.unknown": "Nope.",
        "transport.headers.5.value": "Out of range.",
        modality: "Cannot change.",
      },
      values,
      codecs,
    );
    expect(matched).toEqual([
      ["name", "Name is taken."],
      ["transport.url", "Use https."],
      ["transport.query.1.key", "Bad key."],
      ["transport.headers.0.value", "Unknown secret."],
      ["transport.body", "Must reference {{message}}."],
      ["codec.options.deltaPath", "Invalid path."],
      ["variables.0.key", "Duplicate."],
      ["secrets", "Enter every secret again."],
      ["secrets.0.value", "Too long."],
      ["traceConnectionId", "Not a traces connection."],
    ]);
    expect(unmatched).toEqual(["Nope.", "Out of range.", "Cannot change."]);
  });

  it("maps init messages and pre-session paths", () => {
    const values = initialFormValues("voice");
    values.transport.initMessages = [{ value: "" }, { value: "{}" }];
    values.transport.preSessionEnabled = true;
    values.transport.preSession.headers = [{ key: "A", value: "1" }];
    values.transport.preSession.captures = [{ name: "", path: "" }, { name: "id", path: "x" }];
    const { matched } = mapServerFieldErrors(
      {
        "transport.initMessages.0": "Invalid JSON.",
        "transport.preSession.url": "Use https.",
        "transport.preSession.headers.0.key": "Bad.",
        "transport.preSession.captures.0.name": "Bad name.",
        "transport.preSession": "Invalid.",
      },
      values,
      codecs,
    );
    expect(matched).toEqual([
      ["transport.initMessages.1.value", "Invalid JSON."],
      ["transport.preSession.url", "Use https."],
      ["transport.preSession.headers.0.key", "Bad."],
      ["transport.preSession.captures.1.name", "Bad name."],
      ["transport.preSession.url", "Invalid."],
    ]);
  });
});

describe("templateVariables", () => {
  it("always offers the thread id", () => {
    expect(templateVariables(initialFormValues("voice")).map((v) => v.token)).toEqual(["{{run.thread_id}}"]);
  });

  it("lists variables, secrets, captures, the message for text and audio for json_audio", () => {
    const text = applyPreset(initialFormValues(), viviTextPreset);
    expect(templateVariables(text).map((v) => v.token)).toEqual([
      "{{run.thread_id}}",
      "{{var.channelId}}",
      "{{secret.apiKey}}",
      "{{message}}",
    ]);

    const voice = applyPreset(initialFormValues(), relayPreset);
    voice.variables.push({ key: " ", value: "x" });
    expect(templateVariables(voice).map((v) => v.token)).toEqual([
      "{{run.thread_id}}",
      "{{secret.token}}",
      "{{session.sessionId}}",
      "{{audio}}",
    ]);
  });

  it("ignores captures while the pre-session call is off", () => {
    const voice = applyPreset(initialFormValues(), relayPreset);
    voice.transport.preSessionEnabled = false;
    expect(templateVariables(voice).map((v) => v.token)).not.toContain("{{session.sessionId}}");
  });
});
