import type { Metadata } from "next";

import { SectionPlaceholder } from "@/features/shell/components/section-placeholder";

export const metadata: Metadata = { title: "Runs" };

export default function RunsPage() {
  return <SectionPlaceholder id="runs" />;
}
