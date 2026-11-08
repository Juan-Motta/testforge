import { CircleAlertIcon, WrenchIcon } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

import type { ChatEntry, ToolCallView } from "../lib/playground";

function ToolCallItem({ call }: { call: ToolCallView }) {
  return (
    <li>
      <details className="group rounded-md border bg-background text-xs">
        <summary className="flex cursor-pointer list-none items-center gap-2 px-2 py-1.5 outline-none focus-visible:ring-3 focus-visible:ring-ring/50">
          <WrenchIcon aria-hidden className="size-3.5 shrink-0 text-muted-foreground" />
          <code className="min-w-0 truncate font-mono font-medium">{call.name}</code>
          {call.error && (
            <Badge variant="destructive" className="h-4 px-1.5 text-[10px]">
              Error
            </Badge>
          )}
          <span className="ml-auto shrink-0 text-muted-foreground tabular-nums">{call.latency}</span>
        </summary>
        <div className="flex flex-col gap-2 border-t px-2 py-2">
          <div className="flex flex-col gap-1">
            <span className="text-muted-foreground">Arguments</span>
            <pre className="max-h-40 overflow-auto rounded bg-muted px-2 py-1 font-mono whitespace-pre-wrap break-words">
              {call.arguments}
            </pre>
          </div>
          <div className="flex flex-col gap-1">
            <span className="text-muted-foreground">Result</span>
            <pre
              className={cn(
                "max-h-40 overflow-auto rounded px-2 py-1 font-mono whitespace-pre-wrap break-words",
                call.error ? "bg-destructive/10 text-destructive" : "bg-muted",
              )}
            >
              {call.result || "(empty)"}
            </pre>
          </div>
        </div>
      </details>
    </li>
  );
}

/** The playground conversation: user messages, replies with their tool calls and usage, and errors. */
export function ChatTranscript({ entries }: { entries: ChatEntry[] }) {
  return (
    <ol className="flex flex-col gap-3" aria-live="polite">
      {entries.map((entry) => {
        if (entry.role === "user") {
          return (
            <li key={entry.id} className="ml-8 self-end rounded-lg bg-primary px-3 py-2 text-sm whitespace-pre-wrap text-primary-foreground">
              {entry.text}
            </li>
          );
        }
        if (entry.role === "error") {
          return (
            <li
              key={entry.id}
              role="alert"
              className="flex items-start gap-2 rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive"
            >
              <CircleAlertIcon aria-hidden className="mt-0.5 size-4 shrink-0" />
              <span className="min-w-0 break-words">{entry.message}</span>
            </li>
          );
        }
        const { reply } = entry;
        return (
          <li key={entry.id} className="mr-8 flex flex-col gap-2">
            {reply.toolCalls.length > 0 && (
              <ul className="flex flex-col gap-1" aria-label="Tool calls">
                {reply.toolCalls.map((call, index) => (
                  <ToolCallItem key={index} call={call} />
                ))}
              </ul>
            )}
            <p className="rounded-lg border bg-muted/30 px-3 py-2 text-sm whitespace-pre-wrap">
              {reply.message || <span className="text-muted-foreground">Empty reply.</span>}
            </p>
            <span className="text-[11px] text-muted-foreground tabular-nums">{reply.usage}</span>
          </li>
        );
      })}
    </ol>
  );
}
