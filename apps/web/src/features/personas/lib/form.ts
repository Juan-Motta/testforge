import type { Capability, Persona, PersonaInput, Temperament } from "@/lib/api/types";

/** Editor state. `voice.connectionId` is "" when the persona has no voice. */
export type PersonaFormValues = {
  name: string;
  locale: string;
  temperament: Temperament;
  notes: string;
  voice: { connectionId: string; voice: string };
};

export const TEMPERAMENTS: { value: Temperament; label: string; description: string }[] = [
  { value: "polite", label: "Polite", description: "Courteous and patient, follows the agent's lead." },
  { value: "hurried", label: "Hurried", description: "Short on time, wants answers fast and cuts small talk." },
  { value: "skeptical", label: "Skeptical", description: "Doubts claims and asks the agent to back them up." },
  { value: "confused", label: "Confused", description: "Unsure what to ask, needs things explained twice." },
  { value: "angry", label: "Angry", description: "Frustrated and blunt, tests how the agent de-escalates." },
  { value: "friendly", label: "Friendly", description: "Warm and chatty, may drift off topic." },
  { value: "calm", label: "Calm", description: "Even-tempered, answers clearly and waits." },
  { value: "detailed", label: "Detailed", description: "Gives and asks for every detail, precise and thorough." },
];

export const TEMPERAMENT_LABELS = Object.fromEntries(TEMPERAMENTS.map((t) => [t.value, t.label])) as Record<
  Temperament,
  string
>;

export const LOCALE_SUGGESTIONS = ["en-US", "es-MX", "es-ES", "pt-BR"] as const;

const VOICE_CAPABILITIES: readonly Capability[] = ["tts", "realtime"];

/** Connections a persona can speak through. */
export function voiceConnections<T extends { capabilities: Capability[] }>(connections: T[]): T[] {
  return connections.filter((connection) => connection.capabilities.some((c) => VOICE_CAPABILITIES.includes(c)));
}

export function initialPersonaValues(): PersonaFormValues {
  return { name: "", locale: "en-US", temperament: "polite", notes: "", voice: { connectionId: "", voice: "" } };
}

export function personaFormValues(persona: Persona): PersonaFormValues {
  return {
    name: persona.name,
    locale: persona.locale,
    temperament: persona.temperament,
    notes: persona.notes,
    voice: { connectionId: persona.voice?.connectionId ?? "", voice: persona.voice?.voice ?? "" },
  };
}

export function buildPersonaInput(values: PersonaFormValues): PersonaInput {
  return {
    name: values.name.trim(),
    locale: values.locale.trim(),
    temperament: values.temperament,
    notes: values.notes.trim(),
    voice:
      values.voice.connectionId === ""
        ? null
        : { connectionId: values.voice.connectionId, voice: values.voice.voice.trim() },
  };
}

const FIELD_PATHS = new Set(["name", "locale", "temperament", "notes", "voice.connectionId", "voice.voice"]);
const ALIASES: Record<string, string> = { voice: "voice.connectionId" };

/** Splits the API's `error.fields` into errors on a form path and messages with no input. */
export function mapPersonaFieldErrors(fields: Record<string, string>): {
  matched: [string, string][];
  unmatched: string[];
} {
  const matched: [string, string][] = [];
  const unmatched: string[] = [];
  for (const [path, message] of Object.entries(fields)) {
    const formPath = ALIASES[path] ?? (FIELD_PATHS.has(path) ? path : null);
    if (formPath) matched.push([formPath, message]);
    else unmatched.push(message);
  }
  return { matched, unmatched };
}
