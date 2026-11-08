import type { Metadata } from "next";

import { SandboxPage } from "@/features/sandbox/components/sandbox-page";

export const metadata: Metadata = { title: "Sandbox" };

export default function SandboxRoute() {
  return <SandboxPage />;
}
