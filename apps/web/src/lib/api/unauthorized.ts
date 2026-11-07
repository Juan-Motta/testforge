import { isApiError } from "./errors";

/** Mutation/query meta flag: the caller turns 401 into a form error itself. */
export type UnauthorizedMeta = { handlesUnauthorized?: boolean };

/**
 * A 401 normally means the session ended, so the app sends the user to log in.
 * Log-in and sign-up get 401 for wrong credentials and opt out via meta.
 */
export function shouldRedirectToLogin(
  error: unknown,
  meta: Record<string, unknown> | undefined,
): boolean {
  if (!isApiError(error) || error.status !== 401) return false;
  return (meta as UnauthorizedMeta | undefined)?.handlesUnauthorized !== true;
}
