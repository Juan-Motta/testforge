"use client";

import { CircleCheckIcon, CircleXIcon, CopyIcon, ZapIcon } from "lucide-react";
import { useEffect, useState } from "react";

import { FormError } from "@/components/form-error";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Field, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Spinner } from "@/components/ui/spinner";
import { errorMessage } from "@/lib/api/errors";
import type { Modality, ProbeRequest, ProbeResult } from "@/lib/api/types";
import { cn } from "@/lib/utils";

import { useProbeAgent } from "../api/agents";
import {
  DEFAULT_PROBE_DURATION,
  DEFAULT_PROBE_MESSAGE,
  PROBE_DURATIONS,
  PROBE_EVENT_COLORS,
  formatProbeTime,
} from "../lib/display";
import { copyText } from "./copy-text";

/** Seconds since `startedAt`, ticking while it is set. */
function useElapsedSeconds(startedAt: number | null): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (startedAt === null) return;
    const timer = setInterval(() => setNow(Date.now()), 200);
    return () => clearInterval(timer);
  }, [startedAt]);
  return startedAt === null ? 0 : Math.max(0, Math.floor((now - startedAt) / 1000));
}

function CopyableValue({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex min-w-0 flex-col gap-1">
      <span className="text-xs text-muted-foreground">{label}</span>
      <button
        type="button"
        onClick={() => void copyText(value, `${label} copied`)}
        className="group flex min-w-0 items-center gap-2 rounded-md bg-muted px-2 py-1 text-left outline-none hover:bg-muted/70 focus-visible:ring-3 focus-visible:ring-ring/50"
        aria-label={`Copy ${label.toLowerCase()}`}
      >
        <code className="min-w-0 truncate font-mono text-xs">{value}</code>
        <CopyIcon aria-hidden className="ml-auto size-3.5 shrink-0 text-muted-foreground" />
      </button>
    </div>
  );
}

function ProbeOutcome({ result, modality }: { result: ProbeResult; modality: Modality }) {
  return (
    <div className="flex flex-col gap-4" aria-live="polite">
      <div
        className={cn(
          "flex items-start gap-2 rounded-lg border px-3 py-2.5 text-sm",
          result.ok ? "border-success/30 bg-success/5" : "border-destructive/30 bg-destructive/5",
        )}
      >
        {result.ok ? (
          <CircleCheckIcon aria-hidden className="mt-0.5 size-4 shrink-0 text-success" />
        ) : (
          <CircleXIcon aria-hidden className="mt-0.5 size-4 shrink-0 text-destructive" />
        )}
        <div className="flex min-w-0 flex-col gap-0.5">
          <span className="font-medium">{result.ok ? "Probe succeeded" : "Probe failed"}</span>
          <span className="text-muted-foreground">{result.summary}</span>
        </div>
      </div>
      <div className="grid gap-3">
        <CopyableValue label="Thread id" value={result.threadId} />
        <CopyableValue label="Trace session id" value={result.traceSessionId} />
      </div>
      {modality === "voice" ? (
        <dl className="grid gap-3 text-sm">
          <div className="flex items-baseline justify-between gap-2">
            <dt className="text-muted-foreground">Agent audio</dt>
            <dd className="font-mono text-xs tabular-nums">{(result.audioMs / 1000).toFixed(1)} s</dd>
          </div>
          <div className="flex flex-col gap-1">
            <dt className="text-muted-foreground">Transcript</dt>
            <dd className="rounded-md border bg-muted/30 px-2.5 py-2 text-sm whitespace-pre-wrap">
              {result.transcript || <span className="text-muted-foreground">No transcript received.</span>}
            </dd>
          </div>
        </dl>
      ) : (
        <div className="flex flex-col gap-1 text-sm">
          <span className="text-muted-foreground">Reply</span>
          <p className="rounded-md border bg-muted/30 px-2.5 py-2 whitespace-pre-wrap">
            {result.reply || <span className="text-muted-foreground">No reply received.</span>}
          </p>
        </div>
      )}
      <div className="flex flex-col gap-1">
        <span className="text-xs text-muted-foreground">Event log</span>
        <ol className="max-h-80 overflow-auto rounded-lg bg-zinc-950 p-3 font-mono text-[11px] leading-5 text-zinc-300 dark:ring-1 dark:ring-white/10">
          {result.events.length === 0 && <li className="text-zinc-500">No events.</li>}
          {result.events.map((event, index) => (
            <li key={index} className="grid grid-cols-[3.25rem_4.75rem_minmax(0,1fr)] gap-2">
              <span className="text-zinc-500 tabular-nums">{formatProbeTime(event.atMs)}</span>
              <span className={PROBE_EVENT_COLORS[event.kind]}>{event.kind}</span>
              <span className="break-words">{event.detail}</span>
            </li>
          ))}
        </ol>
      </div>
    </div>
  );
}

type ProbePanelProps = {
  slug: string;
  /** Saved agent id; a new agent cannot be probed yet. */
  agentId?: string;
  modality: Modality;
  canProbe: boolean;
  /** The form has unsaved changes, which the probe does not use. */
  dirty: boolean;
};

export function ProbePanel({ slug, agentId, modality, canProbe, dirty }: ProbePanelProps) {
  const probe = useProbeAgent(slug);
  const [message, setMessage] = useState(DEFAULT_PROBE_MESSAGE);
  const [duration, setDuration] = useState(DEFAULT_PROBE_DURATION);
  const [startedAt, setStartedAt] = useState<number | null>(null);
  const elapsed = useElapsedSeconds(startedAt);
  const text = modality === "text";
  const disabled = !agentId || !canProbe;

  function run() {
    if (!agentId || probe.isPending) return;
    const body: ProbeRequest = { durationSeconds: duration };
    if (text) body.message = message.trim() || DEFAULT_PROBE_MESSAGE;
    setStartedAt(Date.now());
    probe.mutate({ id: agentId, body }, { onSettled: () => setStartedAt(null) });
  }

  let hint: string | null = null;
  if (!agentId) hint = "Save the agent to probe it.";
  else if (!canProbe) hint = "Viewers can't run probes.";
  else if (dirty) hint = "The probe uses the saved definition, not unsaved changes.";

  return (
    <Card className="gap-4">
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <ZapIcon aria-hidden className="size-4 text-brand" />
          Probe
        </CardTitle>
        <CardDescription>
          {text
            ? "Sends one message with a fresh thread id and shows what came back."
            : "Connects with a fresh thread id, listens for a few seconds and shows what came back."}
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        <fieldset disabled={disabled || probe.isPending} className="flex min-w-0 flex-col gap-3">
          {text && (
            <Field>
              <FieldLabel htmlFor="probe-message">Message</FieldLabel>
              <Input
                id="probe-message"
                value={message}
                maxLength={2000}
                onChange={(event) => setMessage(event.target.value)}
                onKeyDown={(event) => {
                  // The panel sits inside the agent form: Enter runs the probe instead of saving.
                  if (event.key === "Enter") {
                    event.preventDefault();
                    run();
                  }
                }}
              />
            </Field>
          )}
          <div className="flex items-end gap-2">
            <Field className="w-28 shrink-0">
              <FieldLabel htmlFor="probe-duration">Duration</FieldLabel>
              <Select
                value={String(duration)}
                onValueChange={(value) => setDuration(Number(value))}
                disabled={disabled || probe.isPending}
              >
                <SelectTrigger id="probe-duration" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {PROBE_DURATIONS.map((seconds) => (
                    <SelectItem key={seconds} value={String(seconds)}>
                      {seconds} s
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Field>
            <Button type="button" className="flex-1" onClick={run}>
              {probe.isPending ? <Spinner /> : <ZapIcon />}
              {probe.isPending ? `Probing… ${elapsed}s` : "Run probe"}
            </Button>
          </div>
        </fieldset>
        {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
        {probe.isError && <FormError message={errorMessage(probe.error)} />}
        {probe.data && !probe.isPending && <ProbeOutcome result={probe.data} modality={modality} />}
      </CardContent>
    </Card>
  );
}
