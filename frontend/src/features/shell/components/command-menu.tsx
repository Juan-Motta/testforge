"use client";

import {
  CheckIcon,
  KeyRoundIcon,
  LogOutIcon,
  MonitorIcon,
  MoonIcon,
  SearchIcon,
  SunIcon,
} from "lucide-react";
import { useRouter } from "next/navigation";
import { useTheme } from "next-themes";
import { useEffect, useState } from "react";

import { Button } from "@/components/ui/button";
import {
  Command,
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from "@/components/ui/command";
import { Kbd, KbdGroup } from "@/components/ui/kbd";
import { WorkspaceAvatar } from "@/features/workspaces/components/workspace-avatar";
import { useCurrentWorkspace, useSession } from "@/features/workspaces/session-context";

import { NAV_GROUPS, SECTIONS, sectionHref } from "../lib/navigation";
import { useSignOut } from "./use-sign-out";

/** Search button for the header plus the Cmd/Ctrl+K command palette it opens. */
export function CommandMenuTrigger() {
  const [open, setOpen] = useState(false);
  const router = useRouter();
  const { workspaces } = useSession();
  const current = useCurrentWorkspace();
  const { setTheme } = useTheme();
  const signOut = useSignOut();

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (event.key.toLowerCase() === "k" && (event.metaKey || event.ctrlKey)) {
        event.preventDefault();
        setOpen((value) => !value);
      }
    }
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, []);

  function run(action: () => void) {
    setOpen(false);
    action();
  }

  const sections = [SECTIONS.overview, ...NAV_GROUPS.flatMap((group) => group.items)];

  return (
    <>
      <Button
        variant="outline"
        size="sm"
        onClick={() => setOpen(true)}
        className="gap-2 text-muted-foreground sm:w-56 sm:justify-start"
        aria-label="Open command menu"
      >
        <SearchIcon />
        <span className="hidden sm:inline">Search or jump to…</span>
        <KbdGroup className="ml-auto hidden sm:inline-flex">
          <Kbd>⌘</Kbd>
          <Kbd>K</Kbd>
        </KbdGroup>
      </Button>
      <CommandDialog
        open={open}
        onOpenChange={setOpen}
        title="Command menu"
        description="Jump to a section or workspace, or run a quick action."
      >
        <Command>
        <CommandInput placeholder="Type a command or search…" />
        <CommandList>
          <CommandEmpty>No results found.</CommandEmpty>
          <CommandGroup heading="Go to">
            {sections.map((section) => {
              const Icon = section.icon;
              return (
                <CommandItem
                  key={section.id}
                  value={`go ${section.title}`}
                  keywords={section.keywords}
                  onSelect={() => run(() => router.push(sectionHref(current.slug, section)))}
                >
                  <Icon />
                  {section.title}
                </CommandItem>
              );
            })}
            <CommandItem
              value="go api keys"
              keywords={["tokens", "cli", "mcp"]}
              onSelect={() => run(() => router.push(`/w/${current.slug}/settings/api-keys`))}
            >
              <KeyRoundIcon />
              API keys
            </CommandItem>
          </CommandGroup>
          <CommandSeparator />
          <CommandGroup heading="Workspaces">
            {workspaces.map((workspace) => (
              <CommandItem
                key={workspace.id}
                value={`workspace ${workspace.name} ${workspace.slug}`}
                onSelect={() => run(() => router.push(`/w/${workspace.slug}`))}
              >
                <WorkspaceAvatar name={workspace.name} className="size-5 rounded text-[9px]" />
                {workspace.name}
                {workspace.id === current.id && <CheckIcon className="ml-auto text-brand" />}
              </CommandItem>
            ))}
          </CommandGroup>
          <CommandSeparator />
          <CommandGroup heading="Preferences">
            <CommandItem value="theme light" onSelect={() => run(() => setTheme("light"))}>
              <SunIcon />
              Light theme
            </CommandItem>
            <CommandItem value="theme dark" onSelect={() => run(() => setTheme("dark"))}>
              <MoonIcon />
              Dark theme
            </CommandItem>
            <CommandItem value="theme system" onSelect={() => run(() => setTheme("system"))}>
              <MonitorIcon />
              System theme
            </CommandItem>
            <CommandItem value="log out sign out" onSelect={() => run(signOut)}>
              <LogOutIcon />
              Log out
            </CommandItem>
          </CommandGroup>
        </CommandList>
        </Command>
      </CommandDialog>
    </>
  );
}
