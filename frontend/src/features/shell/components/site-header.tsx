"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { Fragment } from "react";

import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb";
import { Separator } from "@/components/ui/separator";
import { SidebarTrigger } from "@/components/ui/sidebar";
import { useCurrentWorkspace } from "@/features/workspaces/session-context";

import {
  SECTIONS,
  SUBPAGE_TITLES,
  sectionFromPathname,
  sectionHref,
} from "../lib/navigation";
import { CommandMenuTrigger } from "./command-menu";

type Crumb = { label: string; href: string };

function useBreadcrumbs(): Crumb[] {
  const pathname = usePathname();
  const workspace = useCurrentWorkspace();
  const section = sectionFromPathname(pathname);
  const crumbs: Crumb[] = [
    { label: workspace.name, href: sectionHref(workspace.slug, SECTIONS.overview) },
  ];
  if (section.id !== "overview") {
    crumbs.push({ label: section.title, href: sectionHref(workspace.slug, section) });
  }
  const sub = pathname.split("/")[4];
  if (sub && SUBPAGE_TITLES[sub]) {
    crumbs.push({ label: SUBPAGE_TITLES[sub], href: pathname });
  }
  if (crumbs.length === 1) {
    crumbs.push({ label: SECTIONS.overview.title, href: pathname });
  }
  return crumbs;
}

export function SiteHeader() {
  const crumbs = useBreadcrumbs();
  return (
    <header className="sticky top-0 z-10 flex h-14 shrink-0 items-center gap-2 border-b bg-background/80 px-4 backdrop-blur supports-backdrop-filter:bg-background/60">
      <SidebarTrigger className="-ml-1" />
      <Separator orientation="vertical" className="mr-2 data-[orientation=vertical]:h-4 data-[orientation=vertical]:self-center" />
      <Breadcrumb className="min-w-0 flex-1">
        <BreadcrumbList className="flex-nowrap">
          {crumbs.map((crumb, index) => {
            const last = index === crumbs.length - 1;
            return (
              <Fragment key={`${crumb.href}-${crumb.label}`}>
                {index > 0 && (
                  <BreadcrumbSeparator
                    className={index === 1 && crumbs.length > 2 ? "hidden md:block" : undefined}
                  />
                )}
                <BreadcrumbItem className={index === 0 && crumbs.length > 2 ? "hidden md:inline-flex" : "min-w-0"}>
                  {last ? (
                    <BreadcrumbPage className="truncate">{crumb.label}</BreadcrumbPage>
                  ) : (
                    <BreadcrumbLink asChild className="truncate">
                      <Link href={crumb.href}>{crumb.label}</Link>
                    </BreadcrumbLink>
                  )}
                </BreadcrumbItem>
              </Fragment>
            );
          })}
        </BreadcrumbList>
      </Breadcrumb>
      <CommandMenuTrigger />
    </header>
  );
}
