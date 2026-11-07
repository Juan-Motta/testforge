"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";

import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarRail,
  useSidebar,
} from "@/components/ui/sidebar";
import { WorkspaceSwitcher } from "@/features/workspaces/components/workspace-switcher";
import { useCurrentWorkspace } from "@/features/workspaces/session-context";

import {
  NAV_GROUPS,
  SECTIONS,
  sectionFromPathname,
  sectionHref,
  type Section,
} from "../lib/navigation";
import { UserMenu } from "./user-menu";

function NavItem({ section, active, slug }: { section: Section; active: boolean; slug: string }) {
  const { isMobile, setOpenMobile } = useSidebar();
  const Icon = section.icon;
  return (
    <SidebarMenuItem>
      <SidebarMenuButton asChild isActive={active} tooltip={section.title}>
        <Link
          href={sectionHref(slug, section)}
          aria-current={active ? "page" : undefined}
          onClick={() => isMobile && setOpenMobile(false)}
        >
          <Icon />
          <span>{section.title}</span>
        </Link>
      </SidebarMenuButton>
    </SidebarMenuItem>
  );
}

export function AppSidebar() {
  const pathname = usePathname();
  const { slug } = useCurrentWorkspace();
  const active = sectionFromPathname(pathname);

  return (
    <Sidebar collapsible="icon" variant="inset">
      <SidebarHeader>
        <WorkspaceSwitcher />
      </SidebarHeader>
      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupContent>
            <SidebarMenu>
              <NavItem section={SECTIONS.overview} active={active.id === "overview"} slug={slug} />
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
        {NAV_GROUPS.map((group) => (
          <SidebarGroup key={group.label}>
            <SidebarGroupLabel>{group.label}</SidebarGroupLabel>
            <SidebarGroupContent>
              <SidebarMenu>
                {group.items.map((section) => (
                  <NavItem
                    key={section.id}
                    section={section}
                    active={active.id === section.id}
                    slug={slug}
                  />
                ))}
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        ))}
      </SidebarContent>
      <SidebarFooter>
        <UserMenu />
      </SidebarFooter>
      <SidebarRail />
    </Sidebar>
  );
}
