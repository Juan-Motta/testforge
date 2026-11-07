import type { Metadata } from "next";

import { NewAgentPage } from "@/features/agents/components/agent-editor-page";

export const metadata: Metadata = { title: "New agent" };

export default function NewAgentRoute() {
  return <NewAgentPage />;
}
