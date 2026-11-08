"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { api, unwrap } from "@/lib/api/client";
import type { PersonaInput } from "@/lib/api/types";

export const personasQueryKey = (slug: string) => ["workspaces", slug, "personas"] as const;

export function usePersonas(slug: string) {
  return useQuery({
    queryKey: personasQueryKey(slug),
    queryFn: () =>
      unwrap(api.GET("/api/v1/workspaces/{slug}/personas", { params: { path: { slug } } })).then(
        (page) => page.items,
      ),
  });
}

export function useCreatePersona(slug: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: PersonaInput) =>
      unwrap(api.POST("/api/v1/workspaces/{slug}/personas", { params: { path: { slug } }, body })),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: personasQueryKey(slug) }),
  });
}

export function useUpdatePersona(slug: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: PersonaInput }) =>
      unwrap(
        api.PUT("/api/v1/workspaces/{slug}/personas/{id}", {
          params: { path: { slug, id } },
          body,
        }),
      ),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: personasQueryKey(slug) }),
  });
}

export function useDeletePersona(slug: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      unwrap(api.DELETE("/api/v1/workspaces/{slug}/personas/{id}", { params: { path: { slug, id } } })),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: personasQueryKey(slug) }),
  });
}
