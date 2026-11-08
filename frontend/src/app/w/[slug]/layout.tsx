import { cookies } from "next/headers";
import { notFound, redirect } from "next/navigation";
import { Suspense, type ReactNode } from "react";

import { ShellSkeleton } from "@/features/shell/components/shell-skeleton";
import { WorkspaceShell } from "@/features/shell/components/workspace-shell";
import { getMe } from "@/lib/api/server";

type WorkspaceLayoutProps = {
  children: ReactNode;
  params: Promise<{ slug: string }>;
};

export default function WorkspaceLayout({ children, params }: WorkspaceLayoutProps) {
  // Session and params are request-time data, so they stream in behind a boundary.
  return (
    <Suspense fallback={<ShellSkeleton />}>
      <AuthenticatedShell params={params}>{children}</AuthenticatedShell>
    </Suspense>
  );
}

async function AuthenticatedShell({ children, params }: WorkspaceLayoutProps) {
  const [{ slug }, me, cookieStore] = await Promise.all([params, getMe(), cookies()]);
  if (!me) redirect("/login");
  if (!me.workspaces.some((workspace) => workspace.slug === slug)) notFound();

  const sidebarState = cookieStore.get("sidebar_state")?.value;
  return (
    <WorkspaceShell me={me} defaultSidebarOpen={sidebarState !== "false"}>
      {children}
    </WorkspaceShell>
  );
}
