import type { Metadata } from "next";
import { Suspense } from "react";

import { RunsPage } from "@/features/runs/components/runs-page";

export const metadata: Metadata = { title: "Runs" };

export default function RunsRoute() {
  // The filters live in the query string, which is request-time data.
  return (
    <Suspense>
      <RunsPage />
    </Suspense>
  );
}
