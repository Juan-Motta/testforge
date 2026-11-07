import type { Metadata } from "next";

import { SectionPlaceholder } from "@/features/shell/components/section-placeholder";

export const metadata: Metadata = { title: "Agents" };

export default function AgentsPage() {
  return <SectionPlaceholder id="agents" />;
}
