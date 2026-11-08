"use client";

import { useEffect, useReducer, useRef, useState } from "react";

import { runEventsUrl } from "../api/runs";
import { applyRunEvent, INITIAL_LIVE_RUN, parseRunEvent, RUN_EVENT_TYPES } from "../lib/events";

export type StreamState = "connecting" | "open" | "reconnecting" | "closed";

/**
 * Follows `GET .../runs/{id}/events` while `enabled`. The browser reconnects on its own and
 * sends `Last-Event-ID`; replayed events are ignored by the reducer. `onEnd` runs once the
 * `end` event arrives (the stream is then closed).
 */
export function useRunEvents(slug: string, runId: string, enabled: boolean, onEnd: () => void) {
  const [state, dispatch] = useReducer(applyRunEvent, INITIAL_LIVE_RUN);
  const [stream, setStream] = useState<StreamState>("connecting");
  const onEndRef = useRef(onEnd);
  useEffect(() => {
    onEndRef.current = onEnd;
  }, [onEnd]);

  // Once `end` arrived there is nothing left to follow.
  const connect = enabled && !state.ended;
  useEffect(() => {
    if (!connect) return;
    const source = new EventSource(runEventsUrl(slug, runId));
    for (const type of RUN_EVENT_TYPES) {
      source.addEventListener(type, (message: MessageEvent<string>) => {
        const event = parseRunEvent(type, message.data, message.lastEventId);
        if (event) dispatch(event);
        if (type === "end") {
          source.close();
          setStream("closed");
          onEndRef.current();
        }
      });
    }
    source.onopen = () => setStream("open");
    source.onerror = () => setStream(source.readyState === EventSource.CLOSED ? "closed" : "reconnecting");
    return () => source.close();
  }, [slug, runId, connect]);

  return { state, stream };
}
