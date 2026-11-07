import { CompassIcon } from "lucide-react";
import Link from "next/link";

import { EmptyState } from "@/components/empty-state";
import { Button } from "@/components/ui/button";

export default function NotFound() {
  return (
    <div className="flex min-h-svh items-center justify-center px-4">
      <EmptyState
        className="max-w-lg"
        icon={CompassIcon}
        title="Page not found"
        description="This page does not exist, or you do not have access to it."
        action={
          <Button asChild variant="outline">
            <Link href="/">Go to your workspace</Link>
          </Button>
        }
      />
    </div>
  );
}
