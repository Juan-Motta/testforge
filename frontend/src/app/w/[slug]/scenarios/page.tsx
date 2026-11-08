import type { Metadata } from "next";
import { Suspense } from "react";

import { ScenariosPage } from "@/features/scenarios/components/scenarios-page";

export const metadata: Metadata = { title: "Scenarios" };

export default function ScenariosRoute() {
  // The filters live in the query string, which is request-time data.
  return (
    <Suspense>
      <ScenariosPage />
    </Suspense>
  );
}
