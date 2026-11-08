import type { Metadata } from "next";

import { AgentsPage } from "@/features/agents/components/agents-page";

export const metadata: Metadata = { title: "Agents" };

export default function AgentsRoute() {
  return <AgentsPage />;
}
