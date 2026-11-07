"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { api, unwrap } from "@/lib/api/client";
import type { CreateApiKeyRequest } from "@/lib/api/types";

export const apiKeysQueryKey = (slug: string) => ["workspaces", slug, "api-keys"] as const;

export function useApiKeys(slug: string) {
  return useQuery({
    queryKey: apiKeysQueryKey(slug),
    queryFn: () =>
      unwrap(
        api.GET("/api/v1/workspaces/{slug}/api-keys", {
          params: { path: { slug } },
        }),
      ).then((page) => page.items),
  });
}

export function useCreateApiKey(slug: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: CreateApiKeyRequest) =>
      unwrap(
        api.POST("/api/v1/workspaces/{slug}/api-keys", {
          params: { path: { slug } },
          body,
        }),
      ),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: apiKeysQueryKey(slug) }),
  });
}

export function useRevokeApiKey(slug: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (keyId: string) =>
      unwrap(
        api.DELETE("/api/v1/workspaces/{slug}/api-keys/{keyId}", {
          params: { path: { slug, keyId } },
        }),
      ),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: apiKeysQueryKey(slug) }),
  });
}
