import type { Metadata } from "next";
import { Suspense } from "react";

import { ReportsPage } from "@/features/reports/components/reports-page";

export const metadata: Metadata = { title: "Reports" };

export default function ReportsRoute() {
  // The filters live in the query string, which is request-time data.
  return (
    <Suspense>
      <ReportsPage />
    </Suspense>
  );
}
