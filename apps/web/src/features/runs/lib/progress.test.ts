import { describe, expect, it } from "vitest";

import { batchProgress } from "./progress";

describe("batchProgress", () => {
  it("counts finished runs against the total", () => {
    expect(
      batchProgress({ total: 10, counts: { completed: 4, failed: 1, cancelled: 1, running: 2, queued: 2 } }),
    ).toEqual({ finished: 6, total: 10, percent: 60, active: 4, failed: 1, done: false });
  });

  it("is done when nothing is queued or running", () => {
    expect(batchProgress({ total: 2, counts: { completed: 1, interrupted: 1 } })).toEqual({
      finished: 2,
      total: 2,
      percent: 100,
      active: 0,
      failed: 1,
      done: true,
    });
  });

  it("handles an empty batch and missing counts", () => {
    expect(batchProgress({ total: 0, counts: {} })).toMatchObject({ percent: 0, done: true });
  });

  it("counts reruns beyond the original total without passing 100%", () => {
    expect(batchProgress({ total: 2, counts: { completed: 3 } }).percent).toBe(100);
  });
});
