"use client";

import { CloudOffIcon, RotateCwIcon } from "lucide-react";

import { EmptyState } from "@/components/empty-state";
import { Button } from "@/components/ui/button";

export default function ErrorPage({ reset }: { error: Error; reset: () => void }) {
  return (
    <div className="flex min-h-svh items-center justify-center px-4">
      <EmptyState
        className="max-w-lg"
        icon={CloudOffIcon}
        title="Something went wrong"
        description="TestForge could not load this page. The API may be unavailable; try again in a moment."
        action={
          <Button variant="outline" onClick={reset}>
            <RotateCwIcon />
            Try again
          </Button>
        }
      />
    </div>
  );
}
