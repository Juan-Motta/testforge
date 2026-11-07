import "server-only";

import { cookies } from "next/headers";
import createClient from "openapi-fetch";

import { toApiError } from "./errors";
import type { paths } from "./schema";
import type { Me } from "./types";

export const API_URL = process.env.API_URL ?? "http://localhost:8080";

/** Server-side client that forwards the incoming request's cookies to the API. */
export async function serverApi() {
  const cookieStore = await cookies();
  return createClient<paths>({
    baseUrl: API_URL,
    headers: { cookie: cookieStore.toString() },
  });
}

/** Current identity, or `null` when the session is missing or invalid (401). */
export async function getMe(): Promise<Me | null> {
  const api = await serverApi();
  const { data, error, response } = await api.GET("/api/v1/auth/me");
  if (response.status === 401) return null;
  if (!response.ok || !data) throw toApiError(response, error);
  return data;
}
