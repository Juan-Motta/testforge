"use client";

import { EllipsisIcon, FolderIcon, InboxIcon, LayersIcon, PencilIcon, PlusIcon, Trash2Icon, type LucideIcon } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Skeleton } from "@/components/ui/skeleton";
import type { ScenarioGroup } from "@/lib/api/types";
import { cn } from "@/lib/utils";

import { UNGROUPED } from "../lib/filters";

type GroupSidebarProps = {
  groups: ScenarioGroup[] | undefined;
  /** Current group filter: a group id, `none`, or null for all. */
  selected: string | null;
  totalCount: number | undefined;
  ungroupedCount: number | undefined;
  canAuthor: boolean;
  onSelect: (groupId: string | null) => void;
  onCreate: () => void;
  onRename: (group: ScenarioGroup) => void;
  onDelete: (group: ScenarioGroup) => void;
};

function GroupItem({
  icon: Icon,
  label,
  count,
  active,
  onClick,
  menu,
}: {
  icon: LucideIcon;
  label: string;
  count: number | undefined;
  active: boolean;
  onClick: () => void;
  menu?: React.ReactNode;
}) {
  return (
    <li className="group/item relative flex items-center">
      <button
        type="button"
        onClick={onClick}
        aria-current={active ? "true" : undefined}
        className={cn(
          "flex h-8 w-full min-w-0 items-center gap-2 rounded-md px-2 text-left text-sm transition-colors outline-none focus-visible:ring-3 focus-visible:ring-ring/50 [&_svg]:size-4 [&_svg]:shrink-0",
          active ? "bg-muted font-medium text-foreground" : "text-muted-foreground hover:bg-muted/60 hover:text-foreground",
          menu && "pr-14",
        )}
      >
        <Icon aria-hidden />
        <span className="min-w-0 flex-1 truncate">{label}</span>
        <span className={cn("text-xs tabular-nums text-muted-foreground", menu && "absolute right-8")}>
          {count ?? ""}
        </span>
      </button>
      {menu && <div className="absolute right-1">{menu}</div>}
    </li>
  );
}

export function GroupSidebar({
  groups,
  selected,
  totalCount,
  ungroupedCount,
  canAuthor,
  onSelect,
  onCreate,
  onRename,
  onDelete,
}: GroupSidebarProps) {
  return (
    <nav aria-label="Scenario groups" className="flex flex-col gap-3">
      <ul className="flex flex-col gap-0.5">
        <GroupItem icon={LayersIcon} label="All scenarios" count={totalCount} active={selected === null} onClick={() => onSelect(null)} />
        <GroupItem
          icon={InboxIcon}
          label="Ungrouped"
          count={ungroupedCount}
          active={selected === UNGROUPED}
          onClick={() => onSelect(UNGROUPED)}
        />
      </ul>
      <div className="flex items-center justify-between gap-2 px-2">
        <span className="text-xs font-medium text-muted-foreground">Groups</span>
        {canAuthor && (
          <Button type="button" variant="ghost" size="icon-xs" aria-label="New group" onClick={onCreate}>
            <PlusIcon />
          </Button>
        )}
      </div>
      {groups === undefined ? (
        <div className="flex flex-col gap-1.5 px-2" aria-busy aria-label="Loading groups">
          <Skeleton className="h-5 w-32" />
          <Skeleton className="h-5 w-24" />
        </div>
      ) : groups.length === 0 ? (
        <p className="px-2 text-xs text-muted-foreground">
          {canAuthor ? "No groups yet. Use + to create one." : "No groups yet."}
        </p>
      ) : (
        <ul className="flex flex-col gap-0.5">
          {groups.map((group) => (
            <GroupItem
              key={group.id}
              icon={FolderIcon}
              label={group.name}
              count={group.scenarioCount}
              active={selected === group.id}
              onClick={() => onSelect(group.id)}
              menu={
                canAuthor ? (
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button variant="ghost" size="icon-xs" aria-label={`Actions for ${group.name}`}>
                        <EllipsisIcon />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      <DropdownMenuItem onSelect={() => onRename(group)}>
                        <PencilIcon />
                        Rename
                      </DropdownMenuItem>
                      <DropdownMenuSeparator />
                      <DropdownMenuItem variant="destructive" onSelect={() => onDelete(group)}>
                        <Trash2Icon />
                        Delete
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                ) : undefined
              }
            />
          ))}
        </ul>
      )}
    </nav>
  );
}
