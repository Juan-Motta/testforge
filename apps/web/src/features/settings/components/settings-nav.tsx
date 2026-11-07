"use client";

import { KeyRoundIcon, SlidersHorizontalIcon } from "lucide-react";
import Link from "next/link";
import { usePathname } from "next/navigation";

import { useCurrentWorkspace } from "@/features/workspaces/session-context";
import { cn } from "@/lib/utils";

const ITEMS = [
  { title: "General", segment: "", icon: SlidersHorizontalIcon },
  { title: "API keys", segment: "api-keys", icon: KeyRoundIcon },
];

export function SettingsNav() {
  const pathname = usePathname();
  const { slug } = useCurrentWorkspace();
  const base = `/w/${slug}/settings`;

  return (
    <nav aria-label="Settings" className="-mx-1 flex gap-1 overflow-x-auto px-1 lg:mx-0 lg:flex-col lg:px-0">
      {ITEMS.map((item) => {
        const href = item.segment ? `${base}/${item.segment}` : base;
        const active = pathname === href;
        const Icon = item.icon;
        return (
          <Link
            key={item.title}
            href={href}
            aria-current={active ? "page" : undefined}
            className={cn(
              "flex items-center gap-2 rounded-lg px-3 py-2 text-sm whitespace-nowrap text-muted-foreground transition-colors outline-none hover:bg-muted hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50",
              active && "bg-muted font-medium text-foreground",
            )}
          >
            <Icon className="size-4" />
            {item.title}
          </Link>
        );
      })}
    </nav>
  );
}
