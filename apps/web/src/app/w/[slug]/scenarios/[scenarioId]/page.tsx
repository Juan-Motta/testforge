import type { Metadata } from "next";

import { EditScenarioPage } from "@/features/scenarios/components/scenario-editor-page";

export const metadata: Metadata = { title: "Scenario" };

export default function ScenarioRoute() {
  return <EditScenarioPage />;
}
