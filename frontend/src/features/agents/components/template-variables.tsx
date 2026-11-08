"use client";

import { CopyIcon } from "lucide-react";
import { useFormContext, useWatch } from "react-hook-form";

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

import { templateVariables, type AgentFormValues } from "../lib/form";
import { copyText } from "./copy-text";

/** What the current definition can reference; click a token to copy it. */
export function TemplateVariables() {
  const { control } = useFormContext<AgentFormValues>();
  const values = useWatch({ control }) as AgentFormValues;
  const variables = templateVariables(values);

  return (
    <Card className="gap-3">
      <CardHeader>
        <CardTitle>Template variables</CardTitle>
        <CardDescription>Use them in the URL path, query, headers and bodies.</CardDescription>
      </CardHeader>
      <CardContent>
        <ul className="flex flex-col gap-1">
          {variables.map((variable) => (
            <li key={variable.token}>
              <button
                type="button"
                onClick={() => void copyText(variable.token, `Copied ${variable.token}`)}
                className="group flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left outline-none hover:bg-muted focus-visible:ring-3 focus-visible:ring-ring/50"
                aria-label={`Copy ${variable.token}`}
              >
                <span className="flex min-w-0 flex-col">
                  <code className="truncate font-mono text-xs">{variable.token}</code>
                  <span className="truncate text-xs text-muted-foreground">{variable.description}</span>
                </span>
                <CopyIcon aria-hidden className="ml-auto size-3.5 shrink-0 text-muted-foreground opacity-0 group-hover:opacity-100 group-focus-visible:opacity-100" />
              </button>
            </li>
          ))}
        </ul>
      </CardContent>
    </Card>
  );
}
