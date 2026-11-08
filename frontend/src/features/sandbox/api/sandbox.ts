"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { agentQueryKey, agentsQueryKey } from "@/features/agents/api/agents";
import { api, unwrap } from "@/lib/api/client";
import type {
  Agent,
  SandboxAgent,
  SandboxAgentInput,
  SandboxChatRequest,
  SandboxGenerateRequest,
} from "@/lib/api/types";

export const sandboxAgentsQueryKey = (slug: string) => ["workspaces", slug, "sandbox-agents"] as const;
export const sandboxAgentQueryKey = (slug: string, id: string) => [...sandboxAgentsQueryKey(slug), id] as const;
export const sandboxMemoriesQueryKey = (slug: string, id: string, userId: string) =>
  [...sandboxAgentQueryKey(slug, id), "memories", userId] as const;

export function useSandboxAgents(slug: string) {
  return useQuery({
    queryKey: sandboxAgentsQueryKey(slug),
    queryFn: () =>
      unwrap(api.GET("/api/v1/workspaces/{slug}/sandbox-agents", { params: { path: { slug } } })).then(
        (page) => page.items,
      ),
  });
}

export function useSandboxAgent(slug: string, id: string) {
  return useQuery({
    queryKey: sandboxAgentQueryKey(slug, id),
    queryFn: () =>
      unwrap(api.GET("/api/v1/workspaces/{slug}/sandbox-agents/{id}", { params: { path: { slug, id } } })),
  });
}

export function useCreateSandboxAgent(slug: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: SandboxAgentInput) =>
      unwrap(api.POST("/api/v1/workspaces/{slug}/sandbox-agents", { params: { path: { slug } }, body })),
    onSuccess: (agent) => {
      queryClient.setQueryData<SandboxAgent>(sandboxAgentQueryKey(slug, agent.id), agent);
      return queryClient.invalidateQueries({ queryKey: sandboxAgentsQueryKey(slug), exact: true });
    },
  });
}

export function useUpdateSandboxAgent(slug: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: SandboxAgentInput }) =>
      unwrap(api.PUT("/api/v1/workspaces/{slug}/sandbox-agents/{id}", { params: { path: { slug, id } }, body })),
    onSuccess: (agent) => {
      queryClient.setQueryData<SandboxAgent>(sandboxAgentQueryKey(slug, agent.id), agent);
      return queryClient.invalidateQueries({ queryKey: sandboxAgentsQueryKey(slug), exact: true });
    },
  });
}

export function useDeleteSandboxAgent(slug: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      unwrap(api.DELETE("/api/v1/workspaces/{slug}/sandbox-agents/{id}", { params: { path: { slug, id } } })),
    onSuccess: (_, id) => {
      queryClient.removeQueries({ queryKey: sandboxAgentQueryKey(slug, id) });
      return queryClient.invalidateQueries({ queryKey: sandboxAgentsQueryKey(slug), exact: true });
    },
  });
}

export function useSandboxChat(slug: string, id: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: SandboxChatRequest) =>
      unwrap(api.POST("/api/v1/workspaces/{slug}/sandbox-agents/{id}/chat", { params: { path: { slug, id } }, body })),
    // The agent may have remembered something.
    onSuccess: () => queryClient.invalidateQueries({ queryKey: [...sandboxAgentQueryKey(slug, id), "memories"] }),
  });
}

export function useSandboxMemories(slug: string, id: string, userId: string) {
  return useQuery({
    queryKey: sandboxMemoriesQueryKey(slug, id, userId),
    queryFn: () =>
      unwrap(
        api.GET("/api/v1/workspaces/{slug}/sandbox-agents/{id}/memories", {
          params: { path: { slug, id }, query: { userId } },
        }),
      ).then((page) => page.items),
  });
}

export function useDeleteSandboxMemory(slug: string, id: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (memoryId: string) =>
      unwrap(
        api.DELETE("/api/v1/workspaces/{slug}/sandbox-agents/{id}/memories/{memoryId}", {
          params: { path: { slug, id, memoryId } },
        }),
      ),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: [...sandboxAgentQueryKey(slug, id), "memories"] }),
  });
}

export function useLinkSandboxAgent(slug: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      unwrap(api.POST("/api/v1/workspaces/{slug}/sandbox-agents/{id}/link", { params: { path: { slug, id } } })),
    onSuccess: (agent) => {
      queryClient.setQueryData<Agent>(agentQueryKey(slug, agent.id), agent);
      return queryClient.invalidateQueries({ queryKey: agentsQueryKey(slug), exact: true });
    },
  });
}

export function useGenerateSandboxDraft(slug: string) {
  return useMutation({
    mutationFn: (body: SandboxGenerateRequest) =>
      unwrap(api.POST("/api/v1/workspaces/{slug}/sandbox/generate", { params: { path: { slug } }, body })),
  });
}
