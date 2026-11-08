"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { api, unwrap } from "@/lib/api/client";
import { isApiError } from "@/lib/api/errors";
import type { Judgement } from "@/lib/api/types";

import { shouldPollJudgement } from "../lib/judgement";
import { POLL_INTERVAL_MS, runQueryKey, runsQueryKey } from "./runs";

export const judgementQueryKey = (slug: string, runId: string) => [...runQueryKey(slug, runId), "judgement"] as const;

/** The run's judgement (null when there is none yet); polls while it is pending. */
export function useJudgement(slug: string, runId: string, enabled: boolean) {
  return useQuery({
    queryKey: judgementQueryKey(slug, runId),
    queryFn: async (): Promise<Judgement | null> => {
      try {
        return await unwrap(
          api.GET("/api/v1/workspaces/{slug}/runs/{id}/judgement", { params: { path: { slug, id: runId } } }),
        );
      } catch (error) {
        if (isApiError(error) && error.status === 404) return null;
        throw error;
      }
    },
    enabled,
    refetchInterval: (q) => (shouldPollJudgement(q.state.data ?? undefined) ? POLL_INTERVAL_MS : false),
  });
}

export function useRejudgeRun(slug: string, runId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () =>
      unwrap(api.POST("/api/v1/workspaces/{slug}/runs/{id}/judgement", { params: { path: { slug, id: runId } } })),
    onSuccess: (judgement) => {
      queryClient.setQueryData(judgementQueryKey(slug, runId), judgement);
      return queryClient.invalidateQueries({ queryKey: runsQueryKey(slug), refetchType: "active" });
    },
  });
}
