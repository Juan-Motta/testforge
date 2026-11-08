import type {
  JudgementStatus,
  Suggestion,
  SuggestionPriority,
  UnobservableReason,
  Verdict,
} from "@/lib/api/types";

const PENDING: readonly JudgementStatus[] = ["queued", "waiting_traces", "running"];

/** Queued, waiting for traces or running: the judgement can still change. */
export function isJudgementPending(status: JudgementStatus | null | undefined): boolean {
  return status != null && PENDING.includes(status);
}

/** The run detail polls the judgement while it is pending. */
export function shouldPollJudgement(judgement: { status: JudgementStatus } | undefined): boolean {
  return isJudgementPending(judgement?.status);
}

export function pendingJudgementText(status: JudgementStatus): string {
  switch (status) {
    case "queued":
      return "Judgement queued…";
    case "waiting_traces":
      return "Waiting for Langfuse traces…";
    default:
      return "Judging the conversation…";
  }
}

/** Score buckets follow the bands: 90+ passes, 70–89 is partial, below fails. */
export type ScoreTone = "good" | "fair" | "poor" | "none";

export function scoreTone(score: number | null): ScoreTone {
  if (score === null) return "none";
  if (score >= 90) return "good";
  if (score >= 70) return "fair";
  return "poor";
}

export function passedLabel(passed: boolean | null): string {
  if (passed === null) return "Not enough evidence";
  return passed ? "Passed" : "Failed";
}

export const VERDICTS: readonly Verdict[] = ["pass", "partial", "fail", "not_observable"];

export type VerdictTone = "pass" | "partial" | "fail" | "muted";

const VERDICT_INFO: Record<Verdict, { label: string; tone: VerdictTone }> = {
  pass: { label: "Pass", tone: "pass" },
  partial: { label: "Partial", tone: "partial" },
  fail: { label: "Fail", tone: "fail" },
  not_observable: { label: "Not observable", tone: "muted" },
};

export function verdictInfo(verdict: Verdict): { label: string; tone: VerdictTone } {
  return VERDICT_INFO[verdict];
}

const UNOBSERVABLE: Record<NonNullable<UnobservableReason>, string> = {
  traces_unavailable:
    "This criterion needs Langfuse traces, but none were available for this run, so it was not judged.",
  no_timing_data: "This criterion needs turn timing data, which this run does not have, so it was not judged.",
  judge_marked_unobservable: "The judge did not find enough evidence in the conversation to decide this criterion.",
  judge_omitted: "The judge did not return a verdict for this criterion.",
};

export function unobservableReasonText(reason: UnobservableReason): string | null {
  return reason ? UNOBSERVABLE[reason] : null;
}

/** Failure tags that cap a criterion at band 2. */
export const CAPPING_TAGS: readonly string[] = [
  "fabricated_information",
  "wrong_information",
  "instruction_ignored",
  "unsafe_or_policy_violation",
];

export function isCappingTag(tag: string): boolean {
  return CAPPING_TAGS.includes(tag);
}

/** `slow_response` → "Slow response". */
export function formatTag(tag: string): string {
  const words = tag.replaceAll("_", " ");
  return words.charAt(0).toUpperCase() + words.slice(1);
}

/**
 * Transcript index of an evidence ref: `A3` is the agent's 3rd turn and `U2` the
 * user's 2nd. Timing (`T#`) and trace (`R#`) refs have no transcript turn.
 */
export function evidenceRefTarget(ref: string, turns: readonly { role: "user" | "agent" }[]): number | null {
  const match = /^([AU])(\d+)$/i.exec(ref.trim());
  if (!match) return null;
  const role = match[1].toUpperCase() === "A" ? "agent" : "user";
  const nth = Number(match[2]);
  if (nth < 1) return null;
  let seen = 0;
  for (let index = 0; index < turns.length; index++) {
    if (turns[index].role !== role) continue;
    seen++;
    if (seen === nth) return index;
  }
  return null;
}

const PRIORITIES: readonly SuggestionPriority[] = ["high", "medium", "low"];

export function groupSuggestions(
  suggestions: readonly Suggestion[],
): { priority: SuggestionPriority; items: Suggestion[] }[] {
  return PRIORITIES.map((priority) => ({
    priority,
    items: suggestions.filter((suggestion) => suggestion.priority === priority),
  })).filter((group) => group.items.length > 0);
}
