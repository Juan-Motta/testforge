import type {
  Agent,
  AgentCodecSpec,
  AgentInput,
  AgentPreset,
  CodecType,
  KeyValue,
  Modality,
  PreSession,
  Transport,
} from "@/lib/api/types";

export const DEFAULT_TRACE_SESSION_ID = "{{run.thread_id}}";

export type KeyValueRow = { key: string; value: string };
export type CaptureRow = { name: string; path: string };
/** Init messages are wrapped in objects so `useFieldArray` can track them. */
export type InitMessageRow = { value: string };
/** `hint` is the stored hint (`•••• 9f2c`) of a secret loaded from the server; it is never sent. */
export type SecretRow = { name: string; value: string; hint?: string };

export type PreSessionValues = {
  method: PreSession["method"];
  url: string;
  headers: KeyValueRow[];
  body: string;
  captures: CaptureRow[];
};

/**
 * Editor state. It mirrors `AgentInput` so the API's dotted `error.fields` paths
 * (`transport.headers.0.value`) land on form paths, with three differences:
 * blank rows are allowed (and dropped on save), init messages are objects, and the
 * pre-session call is toggled by `preSessionEnabled` instead of being `null`.
 */
export type AgentFormValues = {
  name: string;
  description: string;
  modality: Modality;
  transport: {
    url: string;
    query: KeyValueRow[];
    headers: KeyValueRow[];
    body: string;
    initMessages: InitMessageRow[];
    preSessionEnabled: boolean;
    preSession: PreSessionValues;
  };
  codec: { type: CodecType | ""; options: Record<string, string> };
  variables: KeyValueRow[];
  secrets: SecretRow[];
  /** "" means no trace connection. */
  traceConnectionId: string;
  traceSessionId: string;
};

const emptyPreSession = (): PreSessionValues => ({
  method: "POST",
  url: "",
  headers: [],
  body: "",
  captures: [],
});

const copyRows = (rows: KeyValue[]): KeyValueRow[] => rows.map(({ key, value }) => ({ key, value }));

function transportValues(transport: Transport): AgentFormValues["transport"] {
  const pre = transport.preSession;
  return {
    url: transport.url,
    query: copyRows(transport.query),
    headers: copyRows(transport.headers),
    body: transport.body ?? "",
    initMessages: transport.initMessages.map((value) => ({ value })),
    preSessionEnabled: pre !== null,
    preSession: pre
      ? {
          method: pre.method,
          url: pre.url,
          headers: copyRows(pre.headers),
          body: pre.body ?? "",
          captures: pre.captures.map(({ name, path }) => ({ name, path })),
        }
      : emptyPreSession(),
  };
}

export function initialFormValues(modality: Modality = "voice"): AgentFormValues {
  return {
    name: "",
    description: "",
    modality,
    transport: {
      url: "",
      query: [],
      headers: [],
      body: "",
      initMessages: [],
      preSessionEnabled: false,
      preSession: emptyPreSession(),
    },
    codec: { type: "", options: {} },
    variables: [],
    secrets: [],
    traceConnectionId: "",
    traceSessionId: DEFAULT_TRACE_SESSION_ID,
  };
}

/**
 * Stored definition as form values. Secret values are never returned, so they start
 * empty. Every option of the codec gets a key ("" when unset) so the option inputs do
 * not make a freshly loaded form dirty.
 */
export function formValuesFromAgent(agent: Agent, codecs: AgentCodecSpec[] = []): AgentFormValues {
  const options: Record<string, string> = {};
  for (const field of codecs.find((codec) => codec.codec === agent.codec.type)?.fields ?? []) {
    options[field.key] = "";
  }
  return {
    name: agent.name,
    description: agent.description,
    modality: agent.modality,
    transport: transportValues(agent.transport),
    codec: { type: agent.codec.type, options: { ...options, ...agent.codec.options } },
    variables: copyRows(agent.variables),
    secrets: agent.secrets.map(({ name, hint }) => ({ name, value: "", hint })),
    traceConnectionId: agent.traceConnectionId ?? "",
    traceSessionId: agent.traceSessionId,
  };
}

/**
 * Fills the form from a preset: modality, transport, codec, variables and trace
 * session id are replaced; name (when set), description, trace connection and
 * secrets are kept, and an empty secret row is added for each missing secret name.
 */
export function applyPreset(values: AgentFormValues, preset: AgentPreset): AgentFormValues {
  const existing = new Set(values.secrets.map((secret) => secret.name.trim()));
  const added = preset.secretNames
    .filter((name) => !existing.has(name))
    .map((name): SecretRow => ({ name, value: "" }));
  return {
    ...values,
    name: values.name.trim() === "" ? preset.label : values.name,
    modality: preset.modality,
    transport: transportValues(preset.draft.transport),
    codec: { type: preset.draft.codec.type, options: { ...preset.draft.codec.options } },
    variables: copyRows(preset.draft.variables),
    secrets: [...values.secrets.map((secret) => ({ ...secret })), ...added],
    traceSessionId: preset.draft.traceSessionId,
  };
}

// Blank rows are dropped on save. These predicates are shared with the error mapping,
// which has to translate the API's indexes (into the sent list) back to form indexes.
const keepKeyValue = (row: KeyValueRow) => row.key.trim() !== "" || row.value.trim() !== "";
const keepInitMessage = (row: InitMessageRow) => row.value.trim() !== "";
const keepCapture = (row: CaptureRow) => row.name.trim() !== "" || row.path.trim() !== "";
const keepSecret = (row: SecretRow) => row.name.trim() !== "" || row.value.trim() !== "";

const keyValues = (rows: KeyValueRow[]): KeyValue[] =>
  rows.filter(keepKeyValue).map((row) => ({ key: row.key.trim(), value: row.value }));

/** Indexes, in the form, of the rows that are sent. */
function keptIndexes<T>(rows: T[], keep: (row: T) => boolean): number[] {
  return rows.flatMap((row, index) => (keep(row) ? [index] : []));
}

function buildPreSession(values: PreSessionValues): PreSession {
  return {
    method: values.method,
    url: values.url.trim(),
    headers: keyValues(values.headers),
    body: values.method === "POST" ? values.body.trim() || null : null,
    captures: values.captures
      .filter(keepCapture)
      .map((capture) => ({ name: capture.name.trim(), path: capture.path.trim() })),
  };
}

/**
 * Request body for create and update (PUT replaces the whole definition).
 * Secrets keep their empty value so the server keeps the stored one.
 */
export function buildAgentInput(values: AgentFormValues): AgentInput {
  const { transport } = values;
  const text = values.modality === "text";
  const options: Record<string, string> = {};
  for (const [key, value] of Object.entries(values.codec.options)) {
    // Option inputs register their key even before anything is typed.
    const trimmed = (value ?? "").trim();
    if (trimmed !== "") options[key] = trimmed;
  }
  return {
    name: values.name.trim(),
    description: values.description.trim(),
    modality: values.modality,
    transport: {
      url: transport.url.trim(),
      query: keyValues(transport.query),
      headers: keyValues(transport.headers),
      body: text ? transport.body.trim() || null : null,
      initMessages: text ? [] : transport.initMessages.filter(keepInitMessage).map((row) => row.value.trim()),
      preSession: transport.preSessionEnabled ? buildPreSession(transport.preSession) : null,
    },
    codec: { type: values.codec.type as CodecType, options },
    variables: keyValues(values.variables),
    secrets: values.secrets
      .filter(keepSecret)
      .map((secret) => ({ name: secret.name.trim(), value: secret.value.trim() })),
    traceConnectionId: values.traceConnectionId === "" ? null : values.traceConnectionId,
    traceSessionId: values.traceSessionId.trim() || DEFAULT_TRACE_SESSION_ID,
  };
}

/** Lists that the API indexes, with the form path of each list and the rows it sends. */
function indexedLists(values: AgentFormValues): Record<string, { kept: number[]; leaf?: string }> {
  const { transport } = values;
  return {
    "transport.query": { kept: keptIndexes(transport.query, keepKeyValue) },
    "transport.headers": { kept: keptIndexes(transport.headers, keepKeyValue) },
    "transport.initMessages": { kept: keptIndexes(transport.initMessages, keepInitMessage), leaf: "value" },
    "transport.preSession.headers": { kept: keptIndexes(transport.preSession.headers, keepKeyValue) },
    "transport.preSession.captures": { kept: keptIndexes(transport.preSession.captures, keepCapture) },
    variables: { kept: keptIndexes(values.variables, keepKeyValue) },
    secrets: { kept: keptIndexes(values.secrets, keepSecret) },
  };
}

const ROW_FIELDS: Record<string, readonly string[]> = {
  "transport.query": ["key", "value"],
  "transport.headers": ["key", "value"],
  "transport.initMessages": [],
  "transport.preSession.headers": ["key", "value"],
  "transport.preSession.captures": ["name", "path"],
  variables: ["key", "value"],
  secrets: ["name", "value"],
};

/** Paths that have an input (or a section-level message) in the editor. */
const SCALAR_PATHS = new Set([
  "name",
  "description",
  "transport.url",
  "transport.body",
  "transport.preSession.method",
  "transport.preSession.url",
  "transport.preSession.body",
  "codec.type",
  "traceConnectionId",
  "traceSessionId",
  // List-level messages, shown above the list.
  "transport.query",
  "transport.headers",
  "transport.initMessages",
  "transport.preSession.headers",
  "transport.preSession.captures",
  "variables",
  "secrets",
]);

/** Object-level paths shown on the input that best represents them. */
const ALIASES: Record<string, string> = {
  transport: "transport.url",
  "transport.preSession": "transport.preSession.url",
  codec: "codec.type",
  "codec.options": "codec.type",
};

function mapPath(path: string, values: AgentFormValues, codecs: AgentCodecSpec[]): string | null {
  if (ALIASES[path]) return ALIASES[path];
  if (SCALAR_PATHS.has(path)) return path;

  if (path.startsWith("codec.options.")) {
    const key = path.slice("codec.options.".length);
    const spec = codecs.find((codec) => codec.codec === values.codec.type);
    return spec?.fields.some((field) => field.key === key) ? path : null;
  }

  const lists = indexedLists(values);
  for (const [list, { kept, leaf }] of Object.entries(lists)) {
    if (!path.startsWith(`${list}.`)) continue;
    const [indexPart, field, ...rest] = path.slice(list.length + 1).split(".");
    if (rest.length > 0 || !/^\d+$/.test(indexPart)) return null;
    const formIndex = kept[Number(indexPart)];
    if (formIndex === undefined) return null;
    if (field === undefined) return leaf ? `${list}.${formIndex}.${leaf}` : `${list}.${formIndex}`;
    return ROW_FIELDS[list].includes(field) ? `${list}.${formIndex}.${field}` : null;
  }
  return null;
}

/**
 * Splits the API's `error.fields` into errors on a form path (indexes translated to
 * the form's rows, which include blank rows that were not sent) and messages with no
 * input to show on.
 */
export function mapServerFieldErrors(
  fields: Record<string, string>,
  values: AgentFormValues,
  codecs: AgentCodecSpec[],
): { matched: [string, string][]; unmatched: string[] } {
  const matched: [string, string][] = [];
  const unmatched: string[] = [];
  for (const [path, message] of Object.entries(fields)) {
    const formPath = mapPath(path, values, codecs);
    if (formPath) matched.push([formPath, message]);
    else unmatched.push(message);
  }
  return { matched, unmatched };
}

export type TemplateVariable = { token: string; description: string };

/** Template references the current definition can use, in the order they are shown. */
export function templateVariables(values: AgentFormValues): TemplateVariable[] {
  const list: TemplateVariable[] = [
    { token: "{{run.thread_id}}", description: "Fresh id of each run or probe" },
  ];
  const unique = (names: string[]) => [...new Set(names.map((name) => name.trim()).filter(Boolean))];
  for (const key of unique(values.variables.map((row) => row.key))) {
    list.push({ token: `{{var.${key}}}`, description: "Variable" });
  }
  for (const name of unique(values.secrets.map((row) => row.name))) {
    list.push({ token: `{{secret.${name}}}`, description: "Secret" });
  }
  if (values.transport.preSessionEnabled) {
    for (const name of unique(values.transport.preSession.captures.map((row) => row.name))) {
      list.push({ token: `{{session.${name}}}`, description: "Captured from the handshake" });
    }
  }
  if (values.modality === "text") {
    list.push({ token: "{{message}}", description: "Message sent to the agent (body only)" });
  }
  if (values.codec.type === "json_audio") {
    list.push({ token: "{{audio}}", description: "Base64 audio chunk (send-audio template only)" });
  }
  return list;
}
