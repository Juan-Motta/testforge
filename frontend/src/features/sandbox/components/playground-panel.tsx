"use client";

import { MessageSquarePlusIcon, MessagesSquareIcon, SendIcon } from "lucide-react";
import { useEffect, useRef, useState } from "react";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import { Textarea } from "@/components/ui/textarea";
import { errorMessage } from "@/lib/api/errors";

import { useSandboxChat } from "../api/sandbox";
import { buildChatRequest, MAX_MESSAGE_LENGTH, replyView, type ChatEntry, type NewChatEntry } from "../lib/playground";
import { ChatTranscript } from "./chat-transcript";

type PlaygroundPanelProps = {
  slug: string;
  agentId: string;
  userId: string;
  onUserIdChange: (userId: string) => void;
  /** The form has unsaved changes, which the chat does not use. */
  dirty: boolean;
};

export function PlaygroundPanel({ slug, agentId, userId, onUserIdChange, dirty }: PlaygroundPanelProps) {
  const chat = useSandboxChat(slug, agentId);
  const [entries, setEntries] = useState<ChatEntry[]>([]);
  const [threadId, setThreadId] = useState<string | null>(null);
  const [message, setMessage] = useState("");
  const nextId = useRef(0);
  // Bumped by "New thread" so replies to the previous thread are dropped.
  const generation = useRef(0);
  const endRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    endRef.current?.scrollIntoView({ block: "nearest" });
  }, [entries.length]);

  function push(entry: NewChatEntry) {
    const id = nextId.current++;
    setEntries((current) => [...current, { ...entry, id }]);
  }

  function send() {
    if (chat.isPending || message.trim() === "") return;
    const body = buildChatRequest(message, threadId, userId);
    push({ role: "user", text: body.message });
    setMessage("");
    const sent = generation.current;
    chat.mutate(body, {
      onSuccess: (reply) => {
        if (sent !== generation.current) return;
        setThreadId(reply.threadId);
        push({ role: "assistant", reply: replyView(reply) });
      },
      onError: (error) => {
        if (sent === generation.current) push({ role: "error", message: errorMessage(error) });
      },
    });
  }

  function newThread() {
    generation.current += 1;
    setThreadId(null);
    setEntries([]);
    chat.reset();
  }

  return (
    <Card className="gap-4">
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <MessagesSquareIcon aria-hidden className="size-4 text-brand" />
          Playground
        </CardTitle>
        <CardDescription>Chat with the saved agent and see every tool call.</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        <div className="flex items-end gap-2">
          <Field className="min-w-0 flex-1">
            <FieldLabel htmlFor="playground-user">User id</FieldLabel>
            <Input
              id="playground-user"
              value={userId}
              maxLength={200}
              autoComplete="off"
              spellCheck={false}
              onChange={(event) => onUserIdChange(event.target.value)}
              className="font-mono text-xs md:text-xs"
            />
          </Field>
          <Button type="button" variant="outline" onClick={newThread} disabled={entries.length === 0 && threadId === null}>
            <MessageSquarePlusIcon />
            New thread
          </Button>
        </div>
        <p className="truncate text-xs text-muted-foreground">
          Thread: <code className="font-mono">{threadId ?? "new"}</code>
        </p>
        <div className="max-h-[28rem] min-h-24 overflow-y-auto rounded-lg border bg-muted/10 p-3">
          {entries.length === 0 ? (
            <p className="text-sm text-muted-foreground">Send a message to start a thread.</p>
          ) : (
            <ChatTranscript entries={entries} />
          )}
          {chat.isPending && (
            <p className="mt-3 flex items-center gap-2 text-sm text-muted-foreground" role="status">
              <Spinner /> Thinking…
            </p>
          )}
          <div ref={endRef} />
        </div>
        <div className="flex items-end gap-2">
          <Field className="min-w-0 flex-1">
            <FieldLabel htmlFor="playground-message" className="sr-only">
              Message
            </FieldLabel>
            <Textarea
              id="playground-message"
              value={message}
              maxLength={MAX_MESSAGE_LENGTH}
              placeholder="Message the agent…"
              className="min-h-10"
              onChange={(event) => setMessage(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter" && !event.shiftKey) {
                  event.preventDefault();
                  send();
                }
              }}
            />
          </Field>
          <Button type="button" onClick={send} disabled={chat.isPending || message.trim() === ""} aria-label="Send">
            {chat.isPending ? <Spinner /> : <SendIcon />}
          </Button>
        </div>
        {dirty && <p className="text-xs text-muted-foreground">The playground uses the saved agent, not unsaved changes.</p>}
      </CardContent>
    </Card>
  );
}
