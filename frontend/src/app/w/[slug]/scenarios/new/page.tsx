import type { Metadata } from "next";
import { Suspense } from "react";

import { NewScenarioPage } from "@/features/scenarios/components/scenario-editor-page";

export const metadata: Metadata = { title: "New scenario" };

export default function NewScenarioRoute() {
  // `?agentId=&groupId=` prefill comes from the query string, which is request-time data.
  return (
    <Suspense>
      <NewScenarioPage />
    </Suspense>
  );
}
