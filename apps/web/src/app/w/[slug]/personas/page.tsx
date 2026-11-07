import type { Metadata } from "next";

import { SectionPlaceholder } from "@/features/shell/components/section-placeholder";

export const metadata: Metadata = { title: "Personas" };

export default function PersonasPage() {
  return <SectionPlaceholder id="personas" />;
}
