import type { ReactNode } from "react";

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";

type EditorSectionProps = {
  title: string;
  description?: ReactNode;
  actions?: ReactNode;
  /** Disables every control inside (read-only roles). */
  disabled?: boolean;
  className?: string;
  children: ReactNode;
};

/** A card of the agent editor. Its controls sit in a fieldset so read-only roles get them disabled. */
export function EditorSection({ title, description, actions, disabled, className, children }: EditorSectionProps) {
  return (
    <Card className={cn("gap-4", className)}>
      <CardHeader className="flex flex-row items-start justify-between gap-4">
        <div className="flex min-w-0 flex-col gap-1">
          <CardTitle>{title}</CardTitle>
          {description ? <CardDescription>{description}</CardDescription> : null}
        </div>
        {actions ? <div className="flex shrink-0 gap-2">{actions}</div> : null}
      </CardHeader>
      <CardContent>
        <fieldset disabled={disabled} className="@container flex min-w-0 flex-col gap-5">
          {children}
        </fieldset>
      </CardContent>
    </Card>
  );
}
