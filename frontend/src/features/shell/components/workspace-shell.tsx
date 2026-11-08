"use client";

import type { ReactNode } from "react";

import { SidebarInset, SidebarProvider } from "@/components/ui/sidebar";
import { SessionProvider } from "@/features/workspaces/session-context";
import type { Me } from "@/lib/api/types";

import { AppSidebar } from "./app-sidebar";
import { SiteHeader } from "./site-header";

type WorkspaceShellProps = {
  me: Me;
  defaultSidebarOpen: boolean;
  children: ReactNode;
};

export function WorkspaceShell({ me, defaultSidebarOpen, children }: WorkspaceShellProps) {
  return (
    <SessionProvider me={me}>
      <SidebarProvider defaultOpen={defaultSidebarOpen}>
        <AppSidebar />
        <SidebarInset className="min-w-0">
          <SiteHeader />
          <div className="flex flex-1 flex-col">{children}</div>
        </SidebarInset>
      </SidebarProvider>
    </SessionProvider>
  );
}
