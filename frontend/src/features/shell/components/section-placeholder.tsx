import { EmptyState } from "@/components/empty-state";
import { PageContainer } from "@/components/page-container";
import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";

import { SECTIONS, type SectionId } from "../lib/navigation";

/** Page body for sections that are planned but not built yet. */
export function SectionPlaceholder({ id }: { id: Exclude<SectionId, "overview" | "settings"> }) {
  const section = SECTIONS[id];
  return (
    <PageContainer>
      <PageHeader title={section.title} />
      <EmptyState
        icon={section.icon}
        title={section.emptyTitle ?? section.title}
        description={section.description}
        action={
          <Button disabled aria-disabled>
            Coming soon
          </Button>
        }
      />
    </PageContainer>
  );
}
