import { describe, expect, it } from "vitest";

import {
  evidenceRefTarget,
  groupSuggestions,
  isJudgementPending,
  passedLabel,
  pendingJudgementText,
  scoreTone,
  shouldPollJudgement,
  unobservableReasonText,
  verdictInfo,
  VERDICTS,
} from "./judgement";

const turns = [
  { role: "user" as const },
  { role: "agent" as const },
  { role: "user" as const },
  { role: "agent" as const },
  { role: "agent" as const },
];

describe("evidenceRefTarget", () => {
  it("maps A#/U# to the transcript index of that agent/user turn", () => {
    expect(evidenceRefTarget("A1", turns)).toBe(1);
    expect(evidenceRefTarget("A3", turns)).toBe(4);
    expect(evidenceRefTarget("U2", turns)).toBe(2);
    expect(evidenceRefTarget(" u1 ", turns)).toBe(0);
  });

  it("returns null for out-of-range, timing/trace refs and garbage", () => {
    expect(evidenceRefTarget("A4", turns)).toBeNull();
    expect(evidenceRefTarget("A0", turns)).toBeNull();
    expect(evidenceRefTarget("T2", turns)).toBeNull();
    expect(evidenceRefTarget("R1", turns)).toBeNull();
    expect(evidenceRefTarget("hello", turns)).toBeNull();
    expect(evidenceRefTarget("", turns)).toBeNull();
  });
});

describe("scoreTone", () => {
  it("buckets scores on the band boundaries", () => {
    expect(scoreTone(100)).toBe("good");
    expect(scoreTone(90)).toBe("good");
    expect(scoreTone(89)).toBe("fair");
    expect(scoreTone(70)).toBe("fair");
    expect(scoreTone(69)).toBe("poor");
    expect(scoreTone(0)).toBe("poor");
    expect(scoreTone(null)).toBe("none");
  });
});

describe("passedLabel", () => {
  it("labels passed, failed and not enough evidence", () => {
    expect(passedLabel(true)).toBe("Passed");
    expect(passedLabel(false)).toBe("Failed");
    expect(passedLabel(null)).toBe("Not enough evidence");
  });
});

describe("verdictInfo", () => {
  it("labels every verdict with a distinct tone", () => {
    const tones = new Set(VERDICTS.map((verdict) => verdictInfo(verdict).tone));
    expect(tones.size).toBe(VERDICTS.length);
    expect(verdictInfo("pass").label).toBe("Pass");
    expect(verdictInfo("partial").label).toBe("Partial");
    expect(verdictInfo("fail").label).toBe("Fail");
    expect(verdictInfo("not_observable").label).toBe("Not observable");
  });
});

describe("unobservableReasonText", () => {
  it("explains every reason in plain words", () => {
    expect(unobservableReasonText("traces_unavailable")).toMatch(/Langfuse traces/);
    expect(unobservableReasonText("no_timing_data")).toMatch(/timing/);
    expect(unobservableReasonText("judge_marked_unobservable")).toMatch(/enough evidence/);
    expect(unobservableReasonText("judge_omitted")).toMatch(/did not/);
    expect(unobservableReasonText(null)).toBeNull();
  });
});

describe("judgement polling", () => {
  it("treats queued, waiting_traces and running as pending", () => {
    expect(isJudgementPending("queued")).toBe(true);
    expect(isJudgementPending("waiting_traces")).toBe(true);
    expect(isJudgementPending("running")).toBe(true);
    expect(isJudgementPending("completed")).toBe(false);
    expect(isJudgementPending("failed")).toBe(false);
    expect(isJudgementPending("skipped")).toBe(false);
    expect(isJudgementPending(null)).toBe(false);
    expect(isJudgementPending(undefined)).toBe(false);
  });

  it("polls only while the loaded judgement is pending", () => {
    expect(shouldPollJudgement({ status: "waiting_traces" })).toBe(true);
    expect(shouldPollJudgement({ status: "completed" })).toBe(false);
    expect(shouldPollJudgement(undefined)).toBe(false);
  });

  it("describes each pending status", () => {
    expect(pendingJudgementText("queued")).toMatch(/queued/i);
    expect(pendingJudgementText("waiting_traces")).toBe("Waiting for Langfuse traces…");
    expect(pendingJudgementText("running")).toMatch(/judging/i);
  });
});

describe("groupSuggestions", () => {
  it("groups by priority high → low, keeping order within a group and dropping empty groups", () => {
    const s = (title: string, priority: "high" | "medium" | "low") => ({
      title,
      detail: "",
      area: "prompt" as const,
      priority,
      evidenceRefs: [],
    });
    const groups = groupSuggestions([s("a", "low"), s("b", "high"), s("c", "low"), s("d", "high")]);
    expect(groups.map((group) => group.priority)).toEqual(["high", "low"]);
    expect(groups[0].items.map((item) => item.title)).toEqual(["b", "d"]);
    expect(groups[1].items.map((item) => item.title)).toEqual(["a", "c"]);
    expect(groupSuggestions([])).toEqual([]);
  });
});
