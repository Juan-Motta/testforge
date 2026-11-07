import type {
  CriterionKind,
  CriterionSource,
  HangUpWhen,
  Modality,
  Scenario,
  ScenarioInput,
} from "@/lib/api/types";

export const DEFAULT_END_CONDITIONS = {
  maxDurationSeconds: 360,
  maxSilenceSeconds: 15,
  maxTurns: 20,
  hangUpWhen: "goal_or_goodbye" as HangUpWhen,
};

export type FactRow = { key: string; value: string };
/** `criterionId` is the stored id (null for new criteria); `id` is taken by `useFieldArray`. */
export type CriterionRow = {
  criterionId: string | null;
  text: string;
  kind: CriterionKind;
  source: CriterionSource;
  weight: string;
};
export type InterruptionRow = { agentTurn: string; afterMs: string };
export type VoiceSimulatorMode = "pipeline" | "realtime";

/**
 * Editor state. It mirrors `ScenarioInput` so the API's dotted `error.fields` paths
 * (`criteria.2.weight`) land on form paths, with these differences: numbers are kept
 * as the strings typed, blank fact and interruption rows are allowed (and dropped on
 * save), empty selects are "", and the simulator mode only holds the voice modes
 * (text agents always use `text`).
 */
export type ScenarioFormValues = {
  name: string;
  agentId: string;
  personaId: string;
  groupId: string;
  tags: string[];
  context: string;
  goal: string;
  instructions: string;
  facts: FactRow[];
  criteria: CriterionRow[];
  endConditions: { maxDurationSeconds: string; maxSilenceSeconds: string; maxTurns: string; hangUpWhen: HangUpWhen };
  interruptions: InterruptionRow[];
  simulator: { mode: VoiceSimulatorMode; connectionId: string; model: string };
  judge: { connectionId: string; model: string };
};

export function emptyCriterion(weight = ""): CriterionRow {
  return { criterionId: null, text: "", kind: "must", source: "transcript", weight };
}

export function initialScenarioValues(prefill: { agentId?: string; groupId?: string } = {}): ScenarioFormValues {
  return {
    name: "",
    agentId: prefill.agentId ?? "",
    personaId: "",
    groupId: prefill.groupId ?? "",
    tags: [],
    context: "",
    goal: "",
    instructions: "",
    facts: [],
    criteria: [emptyCriterion("100")],
    endConditions: {
      maxDurationSeconds: String(DEFAULT_END_CONDITIONS.maxDurationSeconds),
      maxSilenceSeconds: String(DEFAULT_END_CONDITIONS.maxSilenceSeconds),
      maxTurns: String(DEFAULT_END_CONDITIONS.maxTurns),
      hangUpWhen: DEFAULT_END_CONDITIONS.hangUpWhen,
    },
    interruptions: [],
    simulator: { mode: "pipeline", connectionId: "", model: "" },
    judge: { connectionId: "", model: "" },
  };
}

export function formValuesFromScenario(scenario: Scenario): ScenarioFormValues {
  const end = scenario.endConditions;
  return {
    name: scenario.name,
    agentId: scenario.agentId,
    personaId: scenario.personaId,
    groupId: scenario.groupId ?? "",
    tags: [...scenario.tags],
    context: scenario.context,
    goal: scenario.goal,
    instructions: scenario.instructions,
    facts: scenario.facts.map(({ key, value }) => ({ key, value })),
    criteria: scenario.criteria.map((criterion) => ({
      criterionId: criterion.id ?? null,
      text: criterion.text,
      kind: criterion.kind,
      source: criterion.source,
      weight: String(criterion.weight),
    })),
    endConditions: {
      maxDurationSeconds: String(end.maxDurationSeconds),
      maxSilenceSeconds: String(end.maxSilenceSeconds),
      maxTurns: String(end.maxTurns),
      hangUpWhen: end.hangUpWhen,
    },
    interruptions: scenario.interruptions.map((row) => ({
      agentTurn: String(row.agentTurn),
      afterMs: String(row.afterMs),
    })),
    simulator: {
      mode: scenario.simulator.mode === "realtime" ? "realtime" : "pipeline",
      connectionId: scenario.simulator.connectionId ?? "",
      model: scenario.simulator.model,
    },
    judge: { connectionId: scenario.judge.connectionId ?? "", model: scenario.judge.model },
  };
}

// Blank rows are dropped on save. These predicates are shared with the error mapping,
// which translates the API's indexes (into the sent list) back to form indexes.
const keepFact = (row: FactRow) => row.key.trim() !== "" || row.value.trim() !== "";
const keepInterruption = (row: InterruptionRow) => row.agentTurn.trim() !== "" || row.afterMs.trim() !== "";
const keepAll = () => true;

/** Integer typed in an input; invalid input becomes 0 so the API reports it on the field. */
function toInt(value: string): number {
  const trimmed = value.trim();
  const number = Number(trimmed);
  return trimmed !== "" && Number.isFinite(number) ? number : 0;
}

const nullable = (value: string) => (value === "" ? null : value);

/**
 * Request body for create and update (PUT replaces the whole scenario). Settings the
 * agent's modality does not use are sent with their defaults: voice agents get no
 * max turns, text agents no max silence, interruptions or voice simulator mode.
 */
export function buildScenarioInput(values: ScenarioFormValues, modality: Modality): ScenarioInput {
  const voice = modality === "voice";
  const end = values.endConditions;
  return {
    name: values.name.trim(),
    agentId: values.agentId,
    personaId: values.personaId,
    groupId: nullable(values.groupId),
    tags: values.tags,
    context: values.context.trim(),
    goal: values.goal.trim(),
    instructions: values.instructions.trim(),
    facts: values.facts.filter(keepFact).map((row) => ({ key: row.key.trim(), value: row.value.trim() })),
    criteria: values.criteria.map((row) => ({
      id: row.criterionId,
      text: row.text.trim(),
      kind: row.kind,
      source: row.source,
      weight: toInt(row.weight),
    })),
    endConditions: {
      maxDurationSeconds: toInt(end.maxDurationSeconds),
      maxSilenceSeconds: voice ? toInt(end.maxSilenceSeconds) : DEFAULT_END_CONDITIONS.maxSilenceSeconds,
      maxTurns: voice ? DEFAULT_END_CONDITIONS.maxTurns : toInt(end.maxTurns),
      hangUpWhen: end.hangUpWhen,
    },
    interruptions: voice
      ? values.interruptions
          .filter(keepInterruption)
          .map((row) => ({ agentTurn: toInt(row.agentTurn), afterMs: toInt(row.afterMs) }))
      : [],
    simulator: {
      mode: voice ? values.simulator.mode : "text",
      connectionId: nullable(values.simulator.connectionId),
      model: values.simulator.model.trim(),
    },
    judge: { connectionId: nullable(values.judge.connectionId), model: values.judge.model.trim() },
  };
}

type IndexedList = { kept: number[]; fields: readonly string[]; aliases?: Record<string, string>; whole: string };

function keptIndexes<T>(rows: T[], keep: (row: T) => boolean): number[] {
  return rows.flatMap((row, index) => (keep(row) ? [index] : []));
}

/** Lists the API indexes: the form rows each one sends, the row inputs, and where whole-row errors go. */
function indexedLists(values: ScenarioFormValues, voice: boolean): Record<string, IndexedList> {
  const lists: Record<string, IndexedList> = {
    facts: { kept: keptIndexes(values.facts, keepFact), fields: ["key", "value"], whole: "key" },
    criteria: {
      kept: keptIndexes(values.criteria, keepAll),
      fields: ["text", "kind", "source", "weight"],
      aliases: { id: "text" },
      whole: "text",
    },
  };
  if (voice) {
    lists.interruptions = {
      kept: keptIndexes(values.interruptions, keepInterruption),
      fields: ["agentTurn", "afterMs"],
      whole: "agentTurn",
    };
  }
  return lists;
}

const COMMON_PATHS = [
  "name",
  "agentId",
  "personaId",
  "groupId",
  "tags",
  "context",
  "goal",
  "instructions",
  "facts",
  "criteria",
  "endConditions.maxDurationSeconds",
  "endConditions.hangUpWhen",
  "simulator.connectionId",
  "simulator.model",
  "judge.connectionId",
  "judge.model",
];
const VOICE_PATHS = ["endConditions.maxSilenceSeconds", "interruptions", "simulator.mode"];
const TEXT_PATHS = ["endConditions.maxTurns"];

/** Object-level paths shown on the input that best represents them. */
const ALIASES: Record<string, string> = {
  endConditions: "endConditions.maxDurationSeconds",
  judge: "judge.connectionId",
};

function mapPath(path: string, values: ScenarioFormValues, modality: Modality | undefined): string | null {
  const voice = modality === "voice";
  const scalars = new Set([...COMMON_PATHS, ...(voice ? VOICE_PATHS : modality === "text" ? TEXT_PATHS : [])]);
  if (ALIASES[path]) return ALIASES[path];
  if (path === "simulator") return voice ? "simulator.mode" : "simulator.connectionId";
  if (scalars.has(path)) return path;
  // Each tag is a chip; its message goes on the tags input.
  if (/^tags\.\d+$/.test(path)) return "tags";

  for (const [list, { kept, fields, aliases, whole }] of Object.entries(indexedLists(values, voice))) {
    if (!path.startsWith(`${list}.`)) continue;
    const [indexPart, field, ...rest] = path.slice(list.length + 1).split(".");
    if (rest.length > 0 || !/^\d+$/.test(indexPart)) return null;
    const formIndex = kept[Number(indexPart)];
    if (formIndex === undefined) return null;
    if (field === undefined) return `${list}.${formIndex}.${whole}`;
    const target = aliases?.[field] ?? field;
    return fields.includes(target) ? `${list}.${formIndex}.${target}` : null;
  }
  return null;
}

/**
 * Splits the API's `error.fields` into errors on a form path (indexes translated to
 * the form's rows, which include blank rows that were not sent) and messages with no
 * input for the agent's modality.
 */
export function mapScenarioFieldErrors(
  fields: Record<string, string>,
  values: ScenarioFormValues,
  modality: Modality | undefined,
): { matched: [string, string][]; unmatched: string[] } {
  const matched: [string, string][] = [];
  const unmatched: string[] = [];
  for (const [path, message] of Object.entries(fields)) {
    const formPath = mapPath(path, values, modality);
    if (formPath) matched.push([formPath, message]);
    else unmatched.push(message);
  }
  return { matched, unmatched };
}
