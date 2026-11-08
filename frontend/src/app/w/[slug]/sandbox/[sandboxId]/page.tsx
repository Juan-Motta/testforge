import type { Metadata } from "next";

import { EditSandboxAgentPage } from "@/features/sandbox/components/sandbox-editor-page";

export const metadata: Metadata = { title: "Sandbox agent" };

export default function SandboxAgentRoute() {
  return <EditSandboxAgentPage />;
}
