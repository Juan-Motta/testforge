"use client";

import createClient from "openapi-fetch";

import { ApiError, toApiError } from "./errors";
import type { paths } from "./schema";

/**
 * Browser client. Requests are same-origin (`/api/...`) and proxied to the Go API
 * by the rewrites in `next.config.ts`, so the HttpOnly `tf_session` cookie is sent
 * automatically.
 */
export const api = createClient<paths>({ baseUrl: "" });

type Result<T> = Promise<{ data?: T; error?: unknown; response: Response }>;

/** Resolves the response data or throws an ApiError (including network failures). */
export async function unwrap<T>(request: Result<T>): Promise<T> {
  let result: Awaited<Result<T>>;
  try {
    result = await request;
  } catch {
    throw new ApiError(
      0,
      "network",
      "Can't reach TestForge. Check your connection and try again.",
    );
  }
  if (!result.response.ok) {
    throw toApiError(result.response, result.error);
  }
  return result.data as T;
}
