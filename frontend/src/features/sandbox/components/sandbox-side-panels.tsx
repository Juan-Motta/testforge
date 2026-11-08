"use client";

import { useState } from "react";

import { DEFAULT_USER_ID } from "../lib/playground";
import { MemoriesPanel } from "./memories-panel";
import { PlaygroundPanel } from "./playground-panel";

type SandboxSidePanelsProps = {
  slug: string;
  agentId: string;
  /** Saved long-term memory setting; the memories list only exists when it is on. */
  longTermMemory: boolean;
  canDeleteMemories: boolean;
  dirty: boolean;
};

/** Playground and memories of a saved agent; they share the user id. */
export function SandboxSidePanels({ slug, agentId, longTermMemory, canDeleteMemories, dirty }: SandboxSidePanelsProps) {
  const [userId, setUserId] = useState(DEFAULT_USER_ID);
  return (
    <>
      <PlaygroundPanel slug={slug} agentId={agentId} userId={userId} onUserIdChange={setUserId} dirty={dirty} />
      {longTermMemory && <MemoriesPanel slug={slug} agentId={agentId} userId={userId} canDelete={canDeleteMemories} />}
    </>
  );
}
