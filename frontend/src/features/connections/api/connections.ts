"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { api, unwrap } from "@/lib/api/client";
import type {
  Connection,
  ConnectionTest,
  CreateConnectionRequest,
  UpdateConnectionRequest,
} from "@/lib/api/types";

export const connectionKindsQueryKey = ["connection-kinds"] as const;
export const connectionsQueryKey = (slug: string) => ["workspaces", slug, "connections"] as const;

/** Provider catalog. It only changes with a deploy, so it is cached for the session. */
export function useConnectionKinds() {
  return useQuery({
    queryKey: connectionKindsQueryKey,
    queryFn: () => unwrap(api.GET("/api/v1/connection-kinds")).then((page) => page.items),
    staleTime: Infinity,
    gcTime: Infinity,
  });
}

export function useConnections(slug: string) {
  return useQuery({
    queryKey: connectionsQueryKey(slug),
    queryFn: () =>
      unwrap(
        api.GET("/api/v1/workspaces/{slug}/connections", {
          params: { path: { slug } },
        }),
      ).then((page) => page.items),
  });
}

export function useCreateConnection(slug: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: CreateConnectionRequest) =>
      unwrap(
        api.POST("/api/v1/workspaces/{slug}/connections", {
          params: { path: { slug } },
          body,
        }),
      ),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: connectionsQueryKey(slug) }),
  });
}

export function useUpdateConnection(slug: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: UpdateConnectionRequest }) =>
      unwrap(
        api.PATCH("/api/v1/workspaces/{slug}/connections/{connectionId}", {
          params: { path: { slug, connectionId: id } },
          body,
        }),
      ),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: connectionsQueryKey(slug) }),
  });
}

export function useDeleteConnection(slug: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      unwrap(
        api.DELETE("/api/v1/workspaces/{slug}/connections/{connectionId}", {
          params: { path: { slug, connectionId: id } },
        }),
      ),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: connectionsQueryKey(slug) }),
  });
}

export function useTestConnection(slug: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string): Promise<ConnectionTest> =>
      unwrap(
        api.POST("/api/v1/workspaces/{slug}/connections/{connectionId}/test", {
          params: { path: { slug, connectionId: id } },
        }),
      ),
    onSuccess: (lastTest, id) => {
      // Show the result right away, then refetch to stay in sync with the server.
      queryClient.setQueryData<Connection[]>(connectionsQueryKey(slug), (items) =>
        items?.map((item) => (item.id === id ? { ...item, lastTest } : item)),
      );
      return queryClient.invalidateQueries({ queryKey: connectionsQueryKey(slug) });
    },
  });
}
