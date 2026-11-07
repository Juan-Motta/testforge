"use client";

import { keepPreviousData, useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { api, unwrap } from "@/lib/api/client";
import type { LaunchRequest, Run, RunBatch } from "@/lib/api/types";

import { filtersToApiQuery, type RunApiQuery, type RunFilters } from "../lib/filters";
import { isActiveStatus, shouldPoll } from "../lib/status";

/** While anything visible is queued or running, lists refresh this often. */
export const POLL_INTERVAL_MS = 3000;
const PAGE_SIZE = 50;
const BATCH_LIMIT = 20;

export const runsQueryKey = (slug: string) => ["workspaces", slug, "runs"] as const;
const runListQueryKey = (slug: string, query: RunApiQuery) => [...runsQueryKey(slug), "list", query] as const;
export const runQueryKey = (slug: string, id: string) => [...runsQueryKey(slug), "detail", id] as const;
export const runBatchesQueryKey = (slug: string) => ["workspaces", slug, "run-batches"] as const;

function useInvalidateRuns(slug: string) {
  const queryClient = useQueryClient();
  return () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: runsQueryKey(slug) }),
      queryClient.invalidateQueries({ queryKey: runBatchesQueryKey(slug) }),
    ]);
}

/** Runs newest first, paginated with `nextCursor`; refreshes while a loaded run is active. */
export function useRuns(slug: string, filters: RunFilters) {
  const query = filtersToApiQuery(filters);
  return useInfiniteQuery({
    queryKey: runListQueryKey(slug, query),
    queryFn: ({ pageParam }) =>
      unwrap(
        api.GET("/api/v1/workspaces/{slug}/runs", {
          params: { path: { slug }, query: { ...query, limit: PAGE_SIZE, ...(pageParam ? { before: pageParam } : {}) } },
        }),
      ),
    initialPageParam: null as string | null,
    getNextPageParam: (page) => page.nextCursor,
    placeholderData: keepPreviousData,
    refetchInterval: (q) =>
      shouldPoll(q.state.data?.pages.flatMap((page) => page.items) ?? []) ? POLL_INTERVAL_MS : false,
  });
}

/** Whether the workspace has any run (one-item list). */
export function useHasRuns(slug: string) {
  return useQuery({
    queryKey: [...runsQueryKey(slug), "any"],
    queryFn: () =>
      unwrap(api.GET("/api/v1/workspaces/{slug}/runs", { params: { path: { slug }, query: { limit: 1 } } })).then(
        (page) => page.items.length > 0,
      ),
  });
}

export function useRunBatches(slug: string) {
  return useQuery({
    queryKey: runBatchesQueryKey(slug),
    queryFn: () =>
      unwrap(
        api.GET("/api/v1/workspaces/{slug}/run-batches", { params: { path: { slug }, query: { limit: BATCH_LIMIT } } }),
      ).then((page) => page.items),
    refetchInterval: (q) => (q.state.data?.some(isBatchActive) ? POLL_INTERVAL_MS : false),
  });
}

function isBatchActive(batch: RunBatch): boolean {
  return (batch.counts.queued ?? 0) + (batch.counts.running ?? 0) > 0;
}

/**
 * One run. While it is active the live stream drives the page; a slow poll keeps the
 * header right even if the stream is unavailable.
 */
export function useRun(slug: string, id: string) {
  return useQuery({
    queryKey: runQueryKey(slug, id),
    queryFn: () => unwrap(api.GET("/api/v1/workspaces/{slug}/runs/{id}", { params: { path: { slug, id } } })),
    refetchInterval: (q) => (q.state.data && isActiveStatus(q.state.data.status) ? POLL_INTERVAL_MS * 2 : false),
  });
}

export function useLaunchRuns(slug: string) {
  const invalidate = useInvalidateRuns(slug);
  return useMutation({
    mutationFn: (body: LaunchRequest) =>
      unwrap(api.POST("/api/v1/workspaces/{slug}/runs", { params: { path: { slug } }, body })),
    onSuccess: invalidate,
  });
}

export function useCancelRun(slug: string) {
  const invalidate = useInvalidateRuns(slug);
  return useMutation({
    mutationFn: (id: string) =>
      unwrap(api.POST("/api/v1/workspaces/{slug}/runs/{id}/cancel", { params: { path: { slug, id } } })),
    onSuccess: invalidate,
  });
}

export function useRerunRun(slug: string) {
  const invalidate = useInvalidateRuns(slug);
  return useMutation({
    mutationFn: (id: string) =>
      unwrap(api.POST("/api/v1/workspaces/{slug}/runs/{id}/rerun", { params: { path: { slug, id } } })),
    onSuccess: invalidate,
  });
}

export function useCancelBatch(slug: string) {
  const invalidate = useInvalidateRuns(slug);
  return useMutation({
    mutationFn: (id: string) =>
      unwrap(api.POST("/api/v1/workspaces/{slug}/run-batches/{id}/cancel", { params: { path: { slug, id } } })),
    onSuccess: invalidate,
  });
}

export function runEventsUrl(slug: string, id: string): string {
  return `/api/v1/workspaces/${encodeURIComponent(slug)}/runs/${encodeURIComponent(id)}/events`;
}

export function runAudioUrl(slug: string, id: string, track: Run["audioTracks"][number]): string {
  return `/api/v1/workspaces/${encodeURIComponent(slug)}/runs/${encodeURIComponent(id)}/audio/${track}`;
}
