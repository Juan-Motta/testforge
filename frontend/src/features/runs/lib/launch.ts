import type { LaunchRequest } from "@/lib/api/types";

export const MAX_REPETITIONS = 10;
export const MAX_CONCURRENCY = 10;
export const DEFAULT_CONCURRENCY = 3;

export type LaunchOptions = { repetitions: number; concurrency: number; label: string };

const plural = (count: number, word: string) => `${count} ${word}${count === 1 ? "" : "s"}`;

/** "3 scenarios × 2 = 6 runs". */
export function launchSummary(scenarioCount: number, repetitions: number): string {
  return `${plural(scenarioCount, "scenario")} × ${repetitions} = ${plural(scenarioCount * repetitions, "run")}`;
}

export function buildLaunchRequest(scenarioIds: string[], options: LaunchOptions): LaunchRequest {
  const request: LaunchRequest = {
    scenarioIds,
    repetitions: options.repetitions,
    concurrency: options.concurrency,
  };
  const label = options.label.trim();
  if (label) request.label = label;
  return request;
}

/** Where a launch problem is fixed: the scenario editor or its persona. */
export type LaunchFix = "scenario" | "persona";

export type ScenarioLaunchError = { scenarioId: string; name: string; message: string; fix: LaunchFix };

export type LaunchErrors = {
  /** Problems of one scenario, in launch order. */
  scenarios: ScenarioLaunchError[];
  /** Errors on the dialog's own inputs. */
  fields: Partial<Record<"repetitions" | "concurrency" | "label", string>>;
  other: string[];
};

const FORM_FIELDS = new Set(["repetitions", "concurrency", "label"]);
const SCENARIO_PATH = /^scenarioIds\.(\d+)$/;

/** A voice scenario's persona without a voice is fixed on the persona. */
function fixFor(message: string): LaunchFix {
  return /persona|voice/i.test(message) ? "persona" : "scenario";
}

/** Splits a 422 `error.fields` into per-scenario problems (named), input errors and the rest. */
export function mapLaunchErrors(
  fields: Record<string, string>,
  scenarioIds: readonly string[],
  scenarios: readonly { id: string; name: string }[],
): LaunchErrors {
  const result: LaunchErrors = { scenarios: [], fields: {}, other: [] };
  const indexed: [number, string][] = [];
  for (const [path, message] of Object.entries(fields)) {
    const match = SCENARIO_PATH.exec(path);
    if (match && Number(match[1]) < scenarioIds.length) indexed.push([Number(match[1]), message]);
    else if (FORM_FIELDS.has(path)) result.fields[path as keyof LaunchErrors["fields"]] = message;
    else result.other.push(message);
  }
  indexed.sort((a, b) => a[0] - b[0]);
  for (const [index, message] of indexed) {
    const scenarioId = scenarioIds[index];
    const name = scenarios.find((scenario) => scenario.id === scenarioId)?.name ?? `Scenario ${index + 1}`;
    result.scenarios.push({ scenarioId, name, message, fix: fixFor(message) });
  }
  return result;
}
