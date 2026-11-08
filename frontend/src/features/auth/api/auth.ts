"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";

import { api, unwrap } from "@/lib/api/client";
import type { UnauthorizedMeta } from "@/lib/api/unauthorized";
import type { LoginRequest, Me, SignupRequest } from "@/lib/api/types";

// Wrong credentials answer 401; the forms show it instead of redirecting to /login.
const handlesUnauthorized = { handlesUnauthorized: true } satisfies UnauthorizedMeta;

export function useLogin() {
  return useMutation({
    mutationFn: (body: LoginRequest) =>
      unwrap(api.POST("/api/v1/auth/login", { body })),
    meta: handlesUnauthorized,
  });
}

export function useSignup() {
  return useMutation({
    mutationFn: (body: SignupRequest) =>
      unwrap(api.POST("/api/v1/auth/signup", { body })),
    meta: handlesUnauthorized,
  });
}

export function useLogout() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => unwrap(api.POST("/api/v1/auth/logout")),
    onSettled: () => queryClient.clear(),
  });
}

/** Where to land after authenticating: the first workspace, or the root fallback. */
export function homePathFor(me: Me): string {
  const first = me.workspaces[0];
  return first ? `/w/${first.slug}` : "/";
}
