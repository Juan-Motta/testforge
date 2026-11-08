import { describe, expect, it } from "vitest";

import { ApiError } from "./errors";
import { shouldRedirectToLogin } from "./unauthorized";

const unauthorized = new ApiError(401, "unauthorized", "Email or password is incorrect.");

describe("shouldRedirectToLogin", () => {
  it("redirects on a 401 from an ordinary request (expired session)", () => {
    expect(shouldRedirectToLogin(unauthorized, undefined)).toBe(true);
  });

  it("does not redirect when the caller handles 401 itself (wrong credentials on log-in)", () => {
    expect(shouldRedirectToLogin(unauthorized, { handlesUnauthorized: true })).toBe(false);
  });

  it("does not redirect on other errors", () => {
    expect(shouldRedirectToLogin(new ApiError(403, "forbidden", "No."), undefined)).toBe(false);
    expect(shouldRedirectToLogin(new Error("boom"), undefined)).toBe(false);
  });
});
