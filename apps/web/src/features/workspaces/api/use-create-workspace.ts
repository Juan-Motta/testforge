"use client";

import { useMutation } from "@tanstack/react-query";

import { api, unwrap } from "@/lib/api/client";
import type { CreateWorkspaceRequest } from "@/lib/api/types";

export function useCreateWorkspace() {
  return useMutation({
    mutationFn: (body: CreateWorkspaceRequest) =>
      unwrap(api.POST("/api/v1/workspaces", { body })),
  });
}
