import type { Metadata } from "next";

import { ConnectionsPage } from "@/features/connections/components/connections-page";

export const metadata: Metadata = { title: "Connections" };

export default function ConnectionsRoute() {
  return <ConnectionsPage />;
}
