import type { Metadata } from "next";

import { WorkspaceOverview } from "@/features/overview/components/workspace-overview";

export const metadata: Metadata = { title: "Overview" };

export default function OverviewPage() {
  return <WorkspaceOverview />;
}
