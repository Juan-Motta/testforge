import type { Metadata } from "next";

import { NewSandboxAgentPage } from "@/features/sandbox/components/sandbox-editor-page";

export const metadata: Metadata = { title: "New sandbox agent" };

export default function NewSandboxAgentRoute() {
  return <NewSandboxAgentPage />;
}
