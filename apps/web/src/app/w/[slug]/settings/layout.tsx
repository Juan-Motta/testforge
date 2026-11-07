import type { ReactNode } from "react";

import { PageContainer } from "@/components/page-container";
import { PageHeader } from "@/components/page-header";
import { SettingsNav } from "@/features/settings/components/settings-nav";

export default function SettingsLayout({ children }: { children: ReactNode }) {
  return (
    <PageContainer>
      <PageHeader title="Settings" description="Manage this workspace and how tools connect to it." />
      <div className="grid gap-8 lg:grid-cols-[12rem_minmax(0,1fr)]">
        <aside>
          <SettingsNav />
        </aside>
        <div className="min-w-0">{children}</div>
      </div>
    </PageContainer>
  );
}
