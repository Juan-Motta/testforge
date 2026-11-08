import { BrainIcon, FlaskConicalIcon, MessagesSquareIcon, WrenchIcon } from "lucide-react";
import Link from "next/link";

import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import type { Connection, SandboxAgent } from "@/lib/api/types";

type SandboxListProps = {
  hrefFor: (id: string) => string;
  agents: SandboxAgent[] | undefined;
  connections: Connection[] | undefined;
  loading: boolean;
};

function CardSkeleton() {
  return (
    <Card className="gap-4 p-5">
      <div className="flex items-center gap-3">
        <Skeleton className="size-9 rounded-lg" />
        <div className="flex flex-col gap-2">
          <Skeleton className="h-4 w-32" />
          <Skeleton className="h-3 w-24" />
        </div>
      </div>
      <Skeleton className="h-4 w-full" />
      <Skeleton className="h-4 w-40" />
    </Card>
  );
}

function memoryLabel(memory: SandboxAgent["memory"]): string | null {
  if (memory.conversation && memory.longTerm) return "Conversation + long-term";
  if (memory.conversation) return "Conversation";
  if (memory.longTerm) return "Long-term";
  return null;
}

export function SandboxList({ hrefFor, agents, connections, loading }: SandboxListProps) {
  if (loading) {
    return (
      <div className="grid gap-4 md:grid-cols-2" aria-busy aria-label="Loading sandbox agents">
        <CardSkeleton />
        <CardSkeleton />
      </div>
    );
  }

  return (
    <ul className="grid gap-4 md:grid-cols-2">
      {agents?.map((agent) => {
        const connection = connections?.find((item) => item.id === agent.connectionId);
        const memory = memoryLabel(agent.memory);
        return (
          <li key={agent.id} className="flex">
            <Card className="relative w-full gap-4 p-5 transition-colors focus-within:ring-3 focus-within:ring-ring/50 hover:bg-muted/30">
              <div className="flex items-start gap-3">
                <span className="flex size-9 shrink-0 items-center justify-center rounded-lg border bg-background shadow-xs [&_svg]:size-4">
                  <FlaskConicalIcon aria-hidden />
                </span>
                <div className="flex min-w-0 flex-1 flex-col gap-0.5">
                  <h3 className="truncate text-sm font-medium">
                    <Link href={hrefFor(agent.id)} className="outline-none after:absolute after:inset-0">
                      {agent.name}
                    </Link>
                  </h3>
                  <p className="truncate text-xs text-muted-foreground">
                    {connection ? `${connection.name} · ` : ""}
                    <span className="font-mono">{agent.model}</span>
                  </p>
                </div>
              </div>
              {agent.description && <p className="line-clamp-2 text-sm text-muted-foreground">{agent.description}</p>}
              <div className="flex flex-wrap items-center gap-x-4 gap-y-2 border-t pt-4 text-xs text-muted-foreground">
                <span className="inline-flex items-center gap-1">
                  <WrenchIcon aria-hidden className="size-3.5" />
                  {agent.tools.length === 1 ? "1 tool" : `${agent.tools.length} tools`}
                </span>
                {memory ? (
                  <span className="inline-flex items-center gap-1">
                    <BrainIcon aria-hidden className="size-3.5" />
                    {memory}
                  </span>
                ) : (
                  <span className="inline-flex items-center gap-1">
                    <MessagesSquareIcon aria-hidden className="size-3.5" />
                    No memory
                  </span>
                )}
              </div>
            </Card>
          </li>
        );
      })}
    </ul>
  );
}
