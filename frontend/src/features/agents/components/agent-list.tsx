"use client";

import { ActivityIcon, BotIcon, KeyRoundIcon } from "lucide-react";
import Link from "next/link";

import { Card } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import type { Agent, AgentCodecSpec, Connection } from "@/lib/api/types";

import { transportHost } from "../lib/display";
import { ModalityBadge } from "./modality-badge";

type AgentListProps = {
  slug: string;
  agents: Agent[] | undefined;
  codecs: AgentCodecSpec[] | undefined;
  connections: Connection[] | undefined;
  loading: boolean;
};

function AgentCardSkeleton() {
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

export function AgentList({ slug, agents, codecs, connections, loading }: AgentListProps) {
  if (loading) {
    return (
      <div className="grid gap-4 md:grid-cols-2" aria-busy aria-label="Loading agents">
        <AgentCardSkeleton />
        <AgentCardSkeleton />
      </div>
    );
  }

  return (
    <ul className="grid gap-4 md:grid-cols-2">
      {agents?.map((agent) => {
        const codecLabel = codecs?.find((codec) => codec.codec === agent.codec.type)?.label ?? agent.codec.type;
        const traceConnection = agent.traceConnectionId
          ? connections?.find((connection) => connection.id === agent.traceConnectionId)
          : undefined;
        const secretCount = agent.secrets.length;
        return (
          <li key={agent.id} className="flex">
            <Card className="relative w-full gap-4 p-5 transition-colors focus-within:ring-3 focus-within:ring-ring/50 hover:bg-muted/30">
              <div className="flex items-start gap-3">
                <span className="flex size-9 shrink-0 items-center justify-center rounded-lg border bg-background shadow-xs [&_svg]:size-4">
                  <BotIcon aria-hidden />
                </span>
                <div className="flex min-w-0 flex-1 flex-col gap-0.5">
                  <h3 className="truncate text-sm font-medium">
                    <Link href={`/w/${slug}/agents/${agent.id}`} className="outline-none after:absolute after:inset-0">
                      {agent.name}
                    </Link>
                  </h3>
                  <p className="truncate text-xs text-muted-foreground">{codecLabel}</p>
                </div>
                <ModalityBadge modality={agent.modality} />
              </div>
              {agent.description && (
                <p className="line-clamp-2 text-sm text-muted-foreground">{agent.description}</p>
              )}
              <div className="flex flex-wrap items-center gap-x-4 gap-y-2 border-t pt-4 text-xs text-muted-foreground">
                <code className="max-w-full truncate rounded-md bg-muted px-1.5 py-0.5 font-mono" aria-label="Transport host">
                  {transportHost(agent.transport.url)}
                </code>
                <span className="inline-flex items-center gap-1">
                  <KeyRoundIcon aria-hidden className="size-3.5" />
                  {secretCount === 1 ? "1 secret" : `${secretCount} secrets`}
                </span>
                {agent.traceConnectionId && (
                  <span className="inline-flex min-w-0 items-center gap-1">
                    <ActivityIcon aria-hidden className="size-3.5 shrink-0" />
                    <span className="truncate">{traceConnection?.name ?? "Langfuse"}</span>
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
