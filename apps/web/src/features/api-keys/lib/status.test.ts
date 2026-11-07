import { describe, expect, it } from "vitest";

import { getApiKeyStatus } from "./status";

const now = new Date("2026-11-07T12:00:00Z");

describe("getApiKeyStatus", () => {
  it("is active without revocation or expiry", () => {
    expect(getApiKeyStatus({ revokedAt: null, expiresAt: null }, now)).toBe("active");
  });

  it("is active before the expiry instant", () => {
    expect(
      getApiKeyStatus({ revokedAt: null, expiresAt: "2026-11-08T12:00:00Z" }, now),
    ).toBe("active");
  });

  it("is expired at or after the expiry instant", () => {
    expect(
      getApiKeyStatus({ revokedAt: null, expiresAt: "2026-11-07T12:00:00Z" }, now),
    ).toBe("expired");
    expect(
      getApiKeyStatus({ revokedAt: null, expiresAt: "2026-01-01T00:00:00Z" }, now),
    ).toBe("expired");
  });

  it("is revoked when revoked, even if also expired", () => {
    expect(
      getApiKeyStatus(
        { revokedAt: "2026-10-01T00:00:00Z", expiresAt: "2026-01-01T00:00:00Z" },
        now,
      ),
    ).toBe("revoked");
  });

  it("treats missing optional fields as not set", () => {
    expect(getApiKeyStatus({}, now)).toBe("active");
  });
});
