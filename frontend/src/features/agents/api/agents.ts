"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { api, unwrap } from "@/lib/api/client";
import type { Agent, AgentInput, ProbeRequest, ProbeResult } from "@/lib/api/types";

export const agentCodecsQueryKey = ["agent-codecs"] as const;
export const agentPresetsQueryKey = ["agent-presets"] as const;
export const agentsQueryKey = (slug: string) => ["workspaces", slug, "agents"] as const;
export const agentQueryKey = (slug: string, id: string) => [...agentsQueryKey(slug), id] as const;

/** Codec catalog. It only changes with a deploy, so it is cached for the session. */
export function useAgentCodecs() {
  return useQuery({
    queryKey: agentCodecsQueryKey,
    queryFn: () => unwrap(api.GET("/api/v1/agent-codecs")).then((page) => page.items),
    staleTime: Infinity,
    gcTime: Infinity,
  });
}

export function useAgentPresets() {
  return useQuery({
    queryKey: agentPresetsQueryKey,
    queryFn: () => unwrap(api.GET("/api/v1/agent-presets")).then((page) => page.items),
    staleTime: Infinity,
    gcTime: Infinity,
  });
}

export function useAgents(slug: string) {
  return useQuery({
    queryKey: agentsQueryKey(slug),
    queryFn: () =>
      unwrap(api.GET("/api/v1/workspaces/{slug}/agents", { params: { path: { slug } } })).then(
        (page) => page.items,
      ),
  });
}

export function useAgent(slug: string, id: string) {
  return useQuery({
    queryKey: agentQueryKey(slug, id),
    queryFn: () =>
      unwrap(
        api.GET("/api/v1/workspaces/{slug}/agents/{agentId}", {
          params: { path: { slug, agentId: id } },
        }),
      ),
  });
}

export function useCreateAgent(slug: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: AgentInput) =>
      unwrap(api.POST("/api/v1/workspaces/{slug}/agents", { params: { path: { slug } }, body })),
    onSuccess: (agent) => {
      queryClient.setQueryData<Agent>(agentQueryKey(slug, agent.id), agent);
      return queryClient.invalidateQueries({ queryKey: agentsQueryKey(slug), exact: true });
    },
  });
}

export function useUpdateAgent(slug: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: AgentInput }) =>
      unwrap(
        api.PUT("/api/v1/workspaces/{slug}/agents/{agentId}", {
          params: { path: { slug, agentId: id } },
          body,
        }),
      ),
    onSuccess: (agent) => {
      queryClient.setQueryData<Agent>(agentQueryKey(slug, agent.id), agent);
      return queryClient.invalidateQueries({ queryKey: agentsQueryKey(slug), exact: true });
    },
  });
}

export function useDeleteAgent(slug: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      unwrap(
        api.DELETE("/api/v1/workspaces/{slug}/agents/{agentId}", {
          params: { path: { slug, agentId: id } },
        }),
      ),
    onSuccess: (_, id) => {
      queryClient.removeQueries({ queryKey: agentQueryKey(slug, id) });
      return queryClient.invalidateQueries({ queryKey: agentsQueryKey(slug), exact: true });
    },
  });
}

export function useProbeAgent(slug: string) {
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: ProbeRequest }): Promise<ProbeResult> =>
      unwrap(
        api.POST("/api/v1/workspaces/{slug}/agents/{agentId}/probe", {
          params: { path: { slug, agentId: id } },
          body,
        }),
      ),
  });
}
