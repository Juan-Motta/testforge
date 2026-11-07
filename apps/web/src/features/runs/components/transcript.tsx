"use client";

import { MessagesSquareIcon } from "lucide-react";
import { useEffect, useRef } from "react";

import { cn } from "@/lib/utils";

import type { LiveTurn } from "../lib/events";
import { formatLatency, formatOffset, isSlowLatency } from "../lib/format";

/** Pixels from the bottom that still count as "following" the conversation. */
const STICK_THRESHOLD = 48;

type TranscriptProps = {
  turns: LiveTurn[];
  live: boolean;
  speaking: boolean;
};

export function Transcript({ turns, live, speaking }: TranscriptProps) {
  const scroller = useRef<HTMLDivElement>(null);
  const following = useRef(true);

  useEffect(() => {
    const element = scroller.current;
    if (element && following.current) element.scrollTop = element.scrollHeight;
  }, [turns, speaking]);

  if (turns.length === 0) {
    return (
      <div className="flex min-h-48 flex-col items-center justify-center gap-2 text-center text-sm text-muted-foreground">
        <MessagesSquareIcon aria-hidden className="size-5" />
        {live ? "Waiting for the conversation to start…" : "No transcript was recorded."}
      </div>
    );
  }

  return (
    <div
      ref={scroller}
      onScroll={(event) => {
        const element = event.currentTarget;
        following.current = element.scrollHeight - element.scrollTop - element.clientHeight < STICK_THRESHOLD;
      }}
      className="flex max-h-[65vh] min-h-48 flex-col gap-3 overflow-y-auto pr-1"
      aria-live={live ? "polite" : undefined}
    >
      <ol className="flex flex-col gap-3">
        {turns.map((turn) => {
          const user = turn.role === "user";
          return (
            <li key={turn.index} className={cn("flex flex-col gap-1", user ? "items-end" : "items-start")}>
              <div className="flex items-center gap-2 text-xs text-muted-foreground">
                <span className="font-medium">{user ? "Simulated user" : "Agent"}</span>
                <span className="tabular-nums">{formatOffset(turn.startMs)}</span>
                {!user && turn.latencyMs !== null && (
                  <span
                    className={cn(
                      "rounded-4xl px-1.5 py-px font-mono text-[11px] tabular-nums",
                      isSlowLatency(turn.latencyMs) ? "bg-destructive/10 text-destructive" : "bg-muted text-muted-foreground",
                    )}
                    title="Time from the end of the user turn to the agent's first reply"
                  >
                    {formatLatency(turn.latencyMs)}
                  </span>
                )}
                {turn.interrupted && (
                  <span className="rounded-4xl bg-warning/10 px-1.5 py-px text-[11px] font-medium text-warning">
                    Interrupted
                  </span>
                )}
              </div>
              <p
                className={cn(
                  "max-w-[85%] rounded-2xl px-3.5 py-2 text-sm whitespace-pre-wrap",
                  user ? "rounded-br-sm bg-primary text-primary-foreground" : "rounded-bl-sm bg-muted",
                  !turn.final && "opacity-80",
                )}
              >
                {turn.text || <span className="italic opacity-70">…</span>}
              </p>
            </li>
          );
        })}
      </ol>
      {speaking && (
        <p className="flex items-center gap-2 text-xs text-muted-foreground" role="status">
          <span aria-hidden className="flex gap-0.5">
            <span className="size-1 animate-bounce rounded-full bg-current [animation-delay:-0.2s]" />
            <span className="size-1 animate-bounce rounded-full bg-current [animation-delay:-0.1s]" />
            <span className="size-1 animate-bounce rounded-full bg-current" />
          </span>
          Agent is speaking…
        </p>
      )}
    </div>
  );
}
