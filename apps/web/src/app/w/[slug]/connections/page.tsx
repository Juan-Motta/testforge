import type { Metadata } from "next";

import { SectionPlaceholder } from "@/features/shell/components/section-placeholder";

export const metadata: Metadata = { title: "Connections" };

export default function ConnectionsPage() {
  return <SectionPlaceholder id="connections" />;
}
