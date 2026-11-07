import type { Metadata } from "next";

import { SectionPlaceholder } from "@/features/shell/components/section-placeholder";

export const metadata: Metadata = { title: "Reports" };

export default function ReportsPage() {
  return <SectionPlaceholder id="reports" />;
}
