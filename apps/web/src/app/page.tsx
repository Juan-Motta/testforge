import { redirect } from "next/navigation";
import { Suspense } from "react";

import { Spinner } from "@/components/ui/spinner";
import { NoWorkspace } from "@/features/workspaces/components/no-workspace";
import { getMe } from "@/lib/api/server";

export default function HomePage() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-svh items-center justify-center">
          <Spinner className="size-5 text-muted-foreground" />
        </div>
      }
    >
      <RedirectToWorkspace />
    </Suspense>
  );
}

async function RedirectToWorkspace() {
  const me = await getMe();
  if (!me) redirect("/login");
  const first = me.workspaces[0];
  if (first) redirect(`/w/${first.slug}`);
  return <NoWorkspace />;
}
