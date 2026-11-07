"use client";

import { useParams } from "next/navigation";
import { createContext, use, type ReactNode } from "react";

import type { Me, Role, Workspace } from "@/lib/api/types";

const SessionContext = createContext<Me | null>(null);

/** Shares the server-validated identity (`GET /api/v1/auth/me`) with client components. */
export function SessionProvider({ me, children }: { me: Me; children: ReactNode }) {
  return <SessionContext value={me}>{children}</SessionContext>;
}

export function useSession(): Me {
  const me = use(SessionContext);
  if (!me) throw new Error("useSession must be used inside <SessionProvider>");
  return me;
}

/** The workspace addressed by the `[slug]` route segment. */
export function useCurrentWorkspace(): Workspace {
  const { workspaces } = useSession();
  const params = useParams<{ slug: string }>();
  const workspace = workspaces.find((w) => w.slug === params.slug);
  if (!workspace) throw new Error(`Workspace "${params.slug}" is not available`);
  return workspace;
}

const MANAGER_ROLES: readonly Role[] = ["owner", "admin"];

export function canManageWorkspace(role: Role): boolean {
  return MANAGER_ROLES.includes(role);
}

export const ROLE_LABELS: Record<Role, string> = {
  owner: "Owner",
  admin: "Admin",
  member: "Member",
  viewer: "Viewer",
};
