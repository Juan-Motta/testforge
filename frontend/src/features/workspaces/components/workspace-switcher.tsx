"use client";

import { CheckIcon, ChevronsUpDownIcon, PlusIcon } from "lucide-react";
import Link from "next/link";
import { useState } from "react";

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  useSidebar,
} from "@/components/ui/sidebar";

import { ROLE_LABELS, useCurrentWorkspace, useSession } from "../session-context";
import { CreateWorkspaceDialog } from "./create-workspace-dialog";
import { WorkspaceAvatar } from "./workspace-avatar";

export function WorkspaceSwitcher() {
  const { workspaces } = useSession();
  const current = useCurrentWorkspace();
  const { isMobile } = useSidebar();
  const [createOpen, setCreateOpen] = useState(false);

  return (
    <SidebarMenu>
      <SidebarMenuItem>
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <SidebarMenuButton
              size="lg"
              className="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground"
            >
              <WorkspaceAvatar name={current.name} />
              <div className="grid flex-1 text-left text-sm leading-tight">
                <span className="truncate font-medium">{current.name}</span>
                <span className="truncate text-xs text-muted-foreground">
                  {ROLE_LABELS[current.role]}
                </span>
              </div>
              <ChevronsUpDownIcon className="ml-auto text-muted-foreground" />
            </SidebarMenuButton>
          </DropdownMenuTrigger>
          <DropdownMenuContent
            className="w-(--radix-dropdown-menu-trigger-width) min-w-60"
            align="start"
            side={isMobile ? "bottom" : "right"}
            sideOffset={4}
          >
            <DropdownMenuLabel className="text-xs text-muted-foreground">
              Workspaces
            </DropdownMenuLabel>
            <DropdownMenuGroup>
              {workspaces.map((workspace) => (
                <DropdownMenuItem key={workspace.id} asChild className="gap-2 p-2">
                  <Link href={`/w/${workspace.slug}`}>
                    <WorkspaceAvatar name={workspace.name} className="size-6 rounded-md text-[10px]" />
                    <span className="flex-1 truncate">{workspace.name}</span>
                    {workspace.id === current.id && <CheckIcon className="text-brand" />}
                  </Link>
                </DropdownMenuItem>
              ))}
            </DropdownMenuGroup>
            <DropdownMenuSeparator />
            <DropdownMenuItem className="gap-2 p-2" onSelect={() => setCreateOpen(true)}>
              <span className="flex size-6 items-center justify-center rounded-md border border-dashed">
                <PlusIcon className="size-3.5" />
              </span>
              <span className="text-muted-foreground">Create workspace</span>
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
        <CreateWorkspaceDialog open={createOpen} onOpenChange={setCreateOpen} />
      </SidebarMenuItem>
    </SidebarMenu>
  );
}
