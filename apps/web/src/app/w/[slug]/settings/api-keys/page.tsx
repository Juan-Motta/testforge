import type { Metadata } from "next";

import { ApiKeysPage } from "@/features/api-keys/components/api-keys-page";

export const metadata: Metadata = { title: "API keys" };

export default function ApiKeysRoute() {
  return <ApiKeysPage />;
}
