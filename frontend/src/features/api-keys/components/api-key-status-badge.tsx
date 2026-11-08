import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

import type { ApiKeyStatus } from "../lib/status";

const STYLES: Record<ApiKeyStatus, { label: string; dot: string; className?: string }> = {
  active: { label: "Active", dot: "bg-success" },
  expired: { label: "Expired", dot: "bg-warning", className: "text-muted-foreground" },
  revoked: { label: "Revoked", dot: "bg-muted-foreground/60", className: "text-muted-foreground" },
};

export function ApiKeyStatusBadge({ status }: { status: ApiKeyStatus }) {
  const style = STYLES[status];
  return (
    <Badge variant="outline" className={cn("gap-1.5 font-normal", style.className)}>
      <span aria-hidden className={cn("size-1.5 rounded-full", style.dot)} />
      {style.label}
    </Badge>
  );
}
