import type { LucideIcon } from "lucide-react";
import type { ReactNode } from "react";

import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty";
import { cn } from "@/lib/utils";

type EmptyStateProps = {
  icon: LucideIcon;
  title: string;
  description: ReactNode;
  action?: ReactNode;
  className?: string;
};

/** Reusable empty state: icon, title, one-line description and an optional action. */
export function EmptyState({
  icon: Icon,
  title,
  description,
  action,
  className,
}: EmptyStateProps) {
  return (
    <Empty
      className={cn(
        "min-h-80 border border-dashed bg-muted/20 py-16",
        className,
      )}
    >
      <EmptyHeader>
        <EmptyMedia className="relative mb-4">
          <span className="absolute inset-0 -z-10 rounded-2xl bg-brand/15 blur-xl" />
          <span className="flex size-12 items-center justify-center rounded-xl border bg-background text-foreground shadow-sm [&_svg]:size-5">
            <Icon />
          </span>
        </EmptyMedia>
        <EmptyTitle className="text-base">{title}</EmptyTitle>
        <EmptyDescription>{description}</EmptyDescription>
      </EmptyHeader>
      {action ? <EmptyContent>{action}</EmptyContent> : null}
    </Empty>
  );
}
