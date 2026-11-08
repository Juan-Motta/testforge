"use client";

import { keepPreviousData, useMutation, useQuery, useQueryClient, type QueryClient } from "@tanstack/react-query";

import { personasQueryKey } from "@/features/personas/api/personas";
import { api, unwrap } from "@/lib/api/client";
import type { Scenario, ScenarioGroupInput, ScenarioInput } from "@/lib/api/types";

import { filtersToApiQuery, NO_FILTERS, type ScenarioFilters } from "../lib/filters";

export const scenarioGroupsQueryKey = (slug: string) => ["workspaces", slug, "scenario-groups"] as const;
export const scenariosQueryKey = (slug: string) => ["workspaces", slug, "scenarios"] as const;
const scenarioListQueryKey = (slug: string, query: Record<string, string>) =>
  [...scenariosQueryKey(slug), "list", query] as const;
export const scenarioQueryKey = (slug: string, id: string) => [...scenariosQueryKey(slug), "detail", id] as const;

/** Lists, group counts and persona usage all change when a scenario is written. */
function invalidateScenarioData(queryClient: QueryClient, slug: string) {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: [...scenariosQueryKey(slug), "list"] }),
    queryClient.invalidateQueries({ queryKey: scenarioGroupsQueryKey(slug) }),
    queryClient.invalidateQueries({ queryKey: personasQueryKey(slug) }),
  ]);
}

export function useScenarioGroups(slug: string) {
  return useQuery({
    queryKey: scenarioGroupsQueryKey(slug),
    queryFn: () =>
      unwrap(api.GET("/api/v1/workspaces/{slug}/scenario-groups", { params: { path: { slug } } })).then(
        (page) => page.items,
      ),
  });
}

export function useCreateScenarioGroup(slug: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: ScenarioGroupInput) =>
      unwrap(api.POST("/api/v1/workspaces/{slug}/scenario-groups", { params: { path: { slug } }, body })),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: scenarioGroupsQueryKey(slug) }),
  });
}

export function useUpdateScenarioGroup(slug: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: ScenarioGroupInput }) =>
      unwrap(
        api.PUT("/api/v1/workspaces/{slug}/scenario-groups/{id}", { params: { path: { slug, id } }, body }),
      ),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: scenarioGroupsQueryKey(slug) }),
  });
}

export function useDeleteScenarioGroup(slug: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      unwrap(api.DELETE("/api/v1/workspaces/{slug}/scenario-groups/{id}", { params: { path: { slug, id } } })),
    // Its scenarios become ungrouped, so stored scenarios change too.
    onSuccess: () => queryClient.invalidateQueries({ queryKey: [...scenariosQueryKey(slug)] }).then(() =>
      queryClient.invalidateQueries({ queryKey: scenarioGroupsQueryKey(slug) }),
    ),
  });
}

export function useScenarios(slug: string, filters: ScenarioFilters = NO_FILTERS) {
  const query = filtersToApiQuery(filters);
  return useQuery({
    queryKey: scenarioListQueryKey(slug, query),
    queryFn: () =>
      unwrap(api.GET("/api/v1/workspaces/{slug}/scenarios", { params: { path: { slug }, query } })).then(
        (page) => page.items,
      ),
    // Keep the previous rows while a new filter loads instead of flashing a skeleton.
    placeholderData: keepPreviousData,
  });
}

export function useScenario(slug: string, id: string) {
  return useQuery({
    queryKey: scenarioQueryKey(slug, id),
    queryFn: () =>
      unwrap(api.GET("/api/v1/workspaces/{slug}/scenarios/{id}", { params: { path: { slug, id } } })),
  });
}

function useStoreScenario(slug: string) {
  const queryClient = useQueryClient();
  return (scenario: Scenario) => {
    queryClient.setQueryData<Scenario>(scenarioQueryKey(slug, scenario.id), scenario);
    return invalidateScenarioData(queryClient, slug);
  };
}

export function useCreateScenario(slug: string) {
  const store = useStoreScenario(slug);
  return useMutation({
    mutationFn: (body: ScenarioInput) =>
      unwrap(api.POST("/api/v1/workspaces/{slug}/scenarios", { params: { path: { slug } }, body })),
    onSuccess: store,
  });
}

export function useUpdateScenario(slug: string) {
  const store = useStoreScenario(slug);
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: ScenarioInput }) =>
      unwrap(api.PUT("/api/v1/workspaces/{slug}/scenarios/{id}", { params: { path: { slug, id } }, body })),
    onSuccess: store,
  });
}

export function useDuplicateScenario(slug: string) {
  const store = useStoreScenario(slug);
  return useMutation({
    mutationFn: (id: string) =>
      unwrap(api.POST("/api/v1/workspaces/{slug}/scenarios/{id}/duplicate", { params: { path: { slug, id } } })),
    onSuccess: store,
  });
}

export function useDeleteScenario(slug: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      unwrap(api.DELETE("/api/v1/workspaces/{slug}/scenarios/{id}", { params: { path: { slug, id } } })),
    onSuccess: (_, id) => {
      queryClient.removeQueries({ queryKey: scenarioQueryKey(slug, id) });
      return invalidateScenarioData(queryClient, slug);
    },
  });
}
