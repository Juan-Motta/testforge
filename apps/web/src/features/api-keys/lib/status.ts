import type { ApiKey } from "@/lib/api/types";

export type ApiKeyStatus = "active" | "revoked" | "expired";

/** Revocation wins over expiry; a key expires at its `expiresAt` instant. */
export function getApiKeyStatus(
  key: Pick<ApiKey, "revokedAt" | "expiresAt">,
  now: Date = new Date(),
): ApiKeyStatus {
  if (key.revokedAt) return "revoked";
  if (key.expiresAt && new Date(key.expiresAt).getTime() <= now.getTime()) {
    return "expired";
  }
  return "active";
}
