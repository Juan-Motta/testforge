import type { Metadata } from "next";

import { EditAgentPage } from "@/features/agents/components/agent-editor-page";

export const metadata: Metadata = { title: "Agent" };

export default function AgentRoute() {
  return <EditAgentPage />;
}
