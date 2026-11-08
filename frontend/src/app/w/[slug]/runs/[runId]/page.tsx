import type { Metadata } from "next";

import { RunDetailPage } from "@/features/runs/components/run-detail-page";

export const metadata: Metadata = { title: "Run" };

export default function RunRoute() {
  return <RunDetailPage />;
}
