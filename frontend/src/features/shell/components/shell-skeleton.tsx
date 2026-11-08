import { Skeleton } from "@/components/ui/skeleton";

/** Placeholder shown while the session and workspace are being resolved. */
export function ShellSkeleton() {
  return (
    <div className="flex min-h-svh bg-sidebar" aria-busy aria-label="Loading workspace">
      <div className="hidden w-64 shrink-0 flex-col gap-6 p-4 md:flex">
        <div className="flex items-center gap-2">
          <Skeleton className="size-8 rounded-lg" />
          <div className="flex flex-1 flex-col gap-1.5">
            <Skeleton className="h-3.5 w-28" />
            <Skeleton className="h-3 w-16" />
          </div>
        </div>
        {[4, 3, 2].map((count, group) => (
          <div key={group} className="flex flex-col gap-3">
            <Skeleton className="h-3 w-14" />
            {Array.from({ length: count }, (_, i) => (
              <Skeleton key={i} className="h-4 w-36" />
            ))}
          </div>
        ))}
      </div>
      <div className="m-0 flex flex-1 flex-col rounded-none bg-background md:m-2 md:ml-0 md:rounded-xl md:shadow-sm">
        <div className="flex h-14 items-center gap-3 border-b px-4">
          <Skeleton className="size-6" />
          <Skeleton className="h-4 w-40" />
        </div>
        <div className="mx-auto flex w-full max-w-6xl flex-col gap-6 px-4 py-8 sm:px-6 lg:px-8">
          <Skeleton className="h-7 w-48" />
          <Skeleton className="h-4 w-80 max-w-full" />
          <Skeleton className="h-64 w-full rounded-xl" />
        </div>
      </div>
    </div>
  );
}
