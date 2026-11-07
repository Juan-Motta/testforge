"use client";

import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Field, FieldDescription, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { ROLE_LABELS, useCurrentWorkspace } from "@/features/workspaces/session-context";
import { formatDate } from "@/lib/format";

export function GeneralSettings() {
  const workspace = useCurrentWorkspace();
  return (
    <section className="flex flex-col gap-6" aria-labelledby="general-heading">
      <div className="flex flex-col gap-1">
        <h2 id="general-heading" className="text-lg font-semibold tracking-tight">
          General
        </h2>
        <p className="text-sm text-muted-foreground">
          Basic details of this workspace. Editing them is coming soon.
        </p>
      </div>
      <Card>
        <CardHeader>
          <CardTitle>Workspace</CardTitle>
          <CardDescription>Created {formatDate(workspace.createdAt)}.</CardDescription>
        </CardHeader>
        <CardContent>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="settings-workspace-name">Name</FieldLabel>
              <Input id="settings-workspace-name" value={workspace.name} readOnly disabled />
            </Field>
            <Field>
              <FieldLabel htmlFor="settings-workspace-slug">URL</FieldLabel>
              <Input id="settings-workspace-slug" value={`/w/${workspace.slug}`} readOnly disabled className="font-mono text-xs" />
              <FieldDescription>Used in links and by API clients to address this workspace.</FieldDescription>
            </Field>
          </FieldGroup>
        </CardContent>
      </Card>
      <Card>
        <CardHeader className="flex flex-row items-center justify-between gap-4">
          <div className="flex flex-col gap-1">
            <CardTitle>Your role</CardTitle>
            <CardDescription>
              Owners and admins manage API keys and settings. Members and viewers have read access to settings.
            </CardDescription>
          </div>
          <Badge variant="secondary">{ROLE_LABELS[workspace.role]}</Badge>
        </CardHeader>
      </Card>
    </section>
  );
}
