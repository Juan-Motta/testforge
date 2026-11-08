import type { Metadata } from "next";

import { PersonasPage } from "@/features/personas/components/personas-page";

export const metadata: Metadata = { title: "Personas" };

export default function PersonasRoute() {
  return <PersonasPage />;
}
