import { Spinner } from "@/components/ui/spinner";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import type { ConnectionTest } from "@/lib/api/types";
import { formatDateTime } from "@/lib/format";
import { cn } from "@/lib/utils";

import { describeLastTest, type LastTestState } from "../lib/status";

const DOTS: Record<LastTestState, string> = {
  never: "bg-muted-foreground/50",
  ok: "bg-success",
  failed: "bg-destructive",
};

type LastTestStatusProps = {
  lastTest: ConnectionTest | null;
  testing?: boolean;
  className?: string;
};

export function LastTestStatus({ lastTest, testing, className }: LastTestStatusProps) {
  if (testing) {
    return (
      <span className={cn("flex items-center gap-2 text-sm text-muted-foreground", className)}>
        <Spinner className="size-3.5" />
        Testing…
      </span>
    );
  }
  const { state, label } = describeLastTest(lastTest);
  const content = (
    <span
      className={cn(
        "flex min-w-0 items-center gap-2 text-sm",
        state === "failed" ? "text-destructive" : "text-muted-foreground",
        className,
      )}
    >
      <span aria-hidden className={cn("size-1.5 shrink-0 rounded-full", DOTS[state])} />
      <span className="truncate">{label}</span>
    </span>
  );
  if (!lastTest) return content;
  return (
    <Tooltip>
      <TooltipTrigger asChild>{content}</TooltipTrigger>
      <TooltipContent className="max-w-xs">
        <p>{lastTest.message}</p>
        <p className="opacity-70">Tested {formatDateTime(lastTest.testedAt)}</p>
      </TooltipContent>
    </Tooltip>
  );
}
