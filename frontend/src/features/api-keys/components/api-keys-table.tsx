"use client";

import { BanIcon, EllipsisIcon } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { ApiKey } from "@/lib/api/types";
import { formatDate, formatDateTime, formatRelative } from "@/lib/format";
import { cn } from "@/lib/utils";

import { getApiKeyStatus } from "../lib/status";
import { ApiKeyStatusBadge } from "./api-key-status-badge";

function DateCell({ value, empty, relative }: { value?: string | null; empty: string; relative?: boolean }) {
  if (!value) return <span className="text-muted-foreground">{empty}</span>;
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <time dateTime={value} className="cursor-default">
          {relative ? formatRelative(value) : formatDate(value)}
        </time>
      </TooltipTrigger>
      <TooltipContent>{formatDateTime(value)}</TooltipContent>
    </Tooltip>
  );
}

// Secondary columns hide when the table's container is narrow (container queries),
// so the name, status and actions always stay visible.
const COLUMNS = [
  { label: "Name" },
  { label: "Key", className: "hidden @lg:table-cell" },
  { label: "Created by", className: "hidden @4xl:table-cell" },
  { label: "Created", className: "hidden @3xl:table-cell" },
  { label: "Last used", className: "hidden @md:table-cell" },
  { label: "Expires", className: "hidden @2xl:table-cell" },
  { label: "Status" },
];
const hide = (label: string) => COLUMNS.find((column) => column.label === label)?.className;

type ApiKeysTableProps = {
  keys: ApiKey[] | undefined;
  loading: boolean;
  canManage: boolean;
  onRevoke: (key: ApiKey) => void;
};

export function ApiKeysTable({ keys, loading, canManage, onRevoke }: ApiKeysTableProps) {
  const now = new Date();
  return (
    <div className="@container overflow-hidden rounded-xl border">
      <Table>
        <TableHeader className="bg-muted/40">
          <TableRow className="hover:bg-transparent">
            {COLUMNS.map((column) => (
              <TableHead
                key={column.label}
                className={cn("h-10 px-4 text-xs font-medium text-muted-foreground", column.className)}
              >
                {column.label}
              </TableHead>
            ))}
            {canManage && (
              <TableHead className="w-12 px-4">
                <span className="sr-only">Actions</span>
              </TableHead>
            )}
          </TableRow>
        </TableHeader>
        <TableBody>
          {loading
            ? Array.from({ length: 3 }, (_, row) => (
                <TableRow key={row}>
                  {COLUMNS.map((column) => (
                    <TableCell key={column.label} className={cn("px-4 py-3", column.className)}>
                      <Skeleton className="h-4 w-20" />
                    </TableCell>
                  ))}
                  {canManage && <TableCell className="px-4" />}
                </TableRow>
              ))
            : keys?.map((key) => {
                const status = getApiKeyStatus(key, now);
                const inactive = status !== "active";
                return (
                  <TableRow key={key.id}>
                    <TableCell className={cn("px-4 py-3 font-medium", inactive && "text-muted-foreground")}>
                      {key.name}
                    </TableCell>
                    <TableCell className={cn("px-4 py-3", hide("Key"))}>
                      <code className="rounded-md bg-muted px-1.5 py-0.5 font-mono text-xs">
                        {key.prefix}…
                      </code>
                    </TableCell>
                    <TableCell className={cn("px-4 py-3", hide("Created by"))}>{key.createdBy.name}</TableCell>
                    <TableCell className={cn("px-4 py-3", hide("Created"))}>
                      <DateCell value={key.createdAt} empty="—" />
                    </TableCell>
                    <TableCell className={cn("px-4 py-3", hide("Last used"))}>
                      <DateCell value={key.lastUsedAt} empty="Never" relative />
                    </TableCell>
                    <TableCell className={cn("px-4 py-3", hide("Expires"))}>
                      <DateCell value={key.expiresAt} empty="Never" />
                    </TableCell>
                    <TableCell className="px-4 py-3">
                      <ApiKeyStatusBadge status={status} />
                    </TableCell>
                    {canManage && (
                      <TableCell className="px-4 py-3 text-right">
                        {status !== "revoked" && (
                          <DropdownMenu>
                            <DropdownMenuTrigger asChild>
                              <Button variant="ghost" size="icon-sm" aria-label={`Actions for ${key.name}`}>
                                <EllipsisIcon />
                              </Button>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent align="end">
                              <DropdownMenuItem variant="destructive" onSelect={() => onRevoke(key)}>
                                <BanIcon />
                                Revoke key
                              </DropdownMenuItem>
                            </DropdownMenuContent>
                          </DropdownMenu>
                        )}
                      </TableCell>
                    )}
                  </TableRow>
                );
              })}
        </TableBody>
      </Table>
    </div>
  );
}
