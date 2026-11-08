import { CheckIcon, CircleCheckIcon, CircleHelpIcon, CircleXIcon, EyeOffIcon, MinusIcon, XIcon } from "lucide-react";

import { Spinner } from "@/components/ui/spinner";
import type { JudgementStatus, Verdict } from "@/lib/api/types";
import { cn } from "@/lib/utils";

import { isJudgementPending, passedLabel, scoreTone, verdictInfo, type ScoreTone, type VerdictTone } from "../lib/judgement";

/** Text colors with ≥ 4.5:1 contrast on cards in both themes. */
export const SCORE_TEXT: Record<ScoreTone, string> = {
  good: "text-emerald-700 dark:text-emerald-400",
  fair: "text-amber-700 dark:text-amber-400",
  poor: "text-red-700 dark:text-red-400",
  none: "text-muted-foreground",
};

/** Meter fills for the same buckets. */
export const SCORE_FILL: Record<ScoreTone, string> = {
  good: "bg-emerald-600 dark:bg-emerald-400",
  fair: "bg-amber-500 dark:bg-amber-400",
  poor: "bg-red-600 dark:bg-red-400",
  none: "bg-muted-foreground",
};

export function formatScore(score: number | null): string {
  return score === null ? "—" : String(Math.round(score));
}

export function ScoreValue({ score, className }: { score: number | null; className?: string }) {
  return <span className={cn("font-semibold tabular-nums", SCORE_TEXT[scoreTone(score)], className)}>{formatScore(score)}</span>;
}

export function ScoreMeter({ score, className }: { score: number | null; className?: string }) {
  return (
    <div className={cn("h-1.5 w-full overflow-hidden rounded-full bg-muted", className)} aria-hidden>
      <div className={cn("h-full rounded-full", SCORE_FILL[scoreTone(score)])} style={{ width: `${score ?? 0}%` }} />
    </div>
  );
}

/*
 * Verdict and pass/fail badges differ in hue, fill lightness and icon, so they stay
 * distinguishable without color: light fill (pass), mid fill (partial), dark solid
 * fill (fail) and a dashed outline (not observable).
 */
const VERDICT_STYLES: Record<VerdictTone, string> = {
  pass: "bg-emerald-100 text-emerald-900 dark:bg-emerald-950 dark:text-emerald-200 dark:ring-1 dark:ring-emerald-800",
  partial: "bg-amber-300 text-amber-950 dark:bg-amber-400 dark:text-amber-950",
  fail: "bg-red-700 text-white dark:bg-red-400 dark:text-red-950",
  muted: "border border-dashed border-muted-foreground/60 text-muted-foreground",
};

const VERDICT_ICONS = { pass: CheckIcon, partial: MinusIcon, fail: XIcon, muted: EyeOffIcon } as const;

const badgeBase =
  "inline-flex h-5 w-fit shrink-0 items-center gap-1 rounded-4xl px-2 text-xs font-medium whitespace-nowrap [&>svg]:size-3";

export function VerdictBadge({ verdict, className }: { verdict: Verdict; className?: string }) {
  const info = verdictInfo(verdict);
  const Icon = VERDICT_ICONS[info.tone];
  return (
    <span className={cn(badgeBase, VERDICT_STYLES[info.tone], className)}>
      <Icon aria-hidden />
      {info.label}
    </span>
  );
}

export function PassedBadge({ passed, className }: { passed: boolean | null; className?: string }) {
  const tone: VerdictTone = passed === null ? "muted" : passed ? "pass" : "fail";
  const Icon = passed === null ? CircleHelpIcon : passed ? CircleCheckIcon : CircleXIcon;
  return (
    <span className={cn(badgeBase, "h-6 px-2.5 text-sm [&>svg]:size-3.5", VERDICT_STYLES[tone], className)}>
      <Icon aria-hidden />
      {passedLabel(passed)}
    </span>
  );
}

/** Compact score for lists: spinner while judging, score with ✓/✗, or a dash. */
export function ScoreCell({
  score,
  passed,
  judgementStatus,
}: {
  score: number | null;
  passed: boolean | null;
  judgementStatus: JudgementStatus | null;
}) {
  if (isJudgementPending(judgementStatus)) {
    return (
      <span className="inline-flex items-center justify-end gap-1 text-muted-foreground" title="Judging">
        <Spinner className="size-3.5" aria-label="Judging" />
      </span>
    );
  }
  if (score === null) {
    return (
      <span className="text-muted-foreground" title={judgementStatus ? `Judgement ${judgementStatus}` : "Not judged"}>
        –
      </span>
    );
  }
  return (
    <span className="inline-flex items-center justify-end gap-1" title={passedLabel(passed)}>
      <ScoreValue score={score} />
      {passed === true && <CheckIcon aria-label="Passed" className="size-3.5 text-emerald-700 dark:text-emerald-400" />}
      {passed === false && <XIcon aria-label="Failed" className="size-3.5 text-red-700 dark:text-red-400" />}
    </span>
  );
}
