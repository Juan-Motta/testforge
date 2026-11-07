"use client";

import { ArrowRightIcon, CircleIcon, KeyRoundIcon, type LucideIcon } from "lucide-react";
import Link from "next/link";

import { PageContainer } from "@/components/page-container";
import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { SECTIONS, sectionHref, type Section } from "@/features/shell/lib/navigation";
import { useCurrentWorkspace, useSession } from "@/features/workspaces/session-context";

type Step = {
  section: Section;
  title: string;
  description: string;
  cta: string;
};

const STEPS: Step[] = [
  {
    section: SECTIONS.connections,
    title: "Connect a provider",
    description:
      "Add the LLM, speech and Langfuse credentials TestForge uses to simulate users and pull traces.",
    cta: "Open connections",
  },
  {
    section: SECTIONS.agents,
    title: "Add an agent",
    description:
      "Point TestForge at your voice or text agent over WebSocket or HTTP, with the headers and params it expects.",
    cta: "Open agents",
  },
  {
    section: SECTIONS.scenarios,
    title: "Write a scenario",
    description:
      "Describe a conversation, pick a persona and list the criteria a good answer has to meet.",
    cta: "Open scenarios",
  },
  {
    section: SECTIONS.runs,
    title: "Launch a run",
    description:
      "Run your scenarios against the agent, follow the conversations live and review the evaluation.",
    cta: "Open runs",
  },
];

function StepIcon({ icon: Icon }: { icon: LucideIcon }) {
  return (
    <span className="flex size-9 shrink-0 items-center justify-center rounded-lg border bg-background shadow-xs">
      <Icon className="size-4" />
    </span>
  );
}

export function WorkspaceOverview() {
  const { user } = useSession();
  const workspace = useCurrentWorkspace();
  const firstName = user.name.split(" ")[0];
  const completed = 0;

  return (
    <PageContainer>
      <PageHeader
        title={`Welcome, ${firstName}`}
        description={`Here is how to get ${workspace.name} ready for its first test run.`}
      />

      <Card className="gap-0 py-0">
        <CardHeader className="flex flex-row items-center justify-between gap-4 border-b py-5">
          <div className="flex flex-col gap-1">
            <CardTitle>Getting started</CardTitle>
            <CardDescription>
              Four steps from an empty workspace to your first evaluated conversation.
            </CardDescription>
          </div>
          <div className="hidden shrink-0 items-center gap-3 sm:flex">
            <span className="text-sm text-muted-foreground tabular-nums">
              {completed} of {STEPS.length} done
            </span>
            <div
              className="h-1.5 w-24 overflow-hidden rounded-full bg-muted"
              role="progressbar"
              aria-label="Getting started progress"
              aria-valuemin={0}
              aria-valuemax={STEPS.length}
              aria-valuenow={completed}
            >
              <div
                className="h-full rounded-full bg-brand transition-all"
                style={{ width: `${(completed / STEPS.length) * 100}%` }}
              />
            </div>
          </div>
        </CardHeader>
        <CardContent className="px-0">
          <ol className="divide-y">
            {STEPS.map((step, index) => (
              <li
                key={step.section.id}
                className="flex flex-col gap-4 px-6 py-5 sm:flex-row sm:items-center"
              >
                <div className="flex flex-1 items-start gap-4">
                  <StepIcon icon={step.section.icon} />
                  <div className="flex flex-col gap-1">
                    <p className="flex items-center gap-2 text-sm font-medium">
                      <CircleIcon className="size-3.5 text-muted-foreground" aria-hidden />
                      <span className="sr-only">Not done:</span>
                      <span className="text-muted-foreground tabular-nums">{index + 1}.</span>
                      {step.title}
                    </p>
                    <p className="text-sm text-muted-foreground">{step.description}</p>
                  </div>
                </div>
                <Button variant="outline" size="sm" asChild className="self-start sm:self-center">
                  <Link href={sectionHref(workspace.slug, step.section)}>
                    {step.cta}
                    <ArrowRightIcon data-icon="inline-end" />
                  </Link>
                </Button>
              </li>
            ))}
          </ol>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="flex flex-row items-start gap-4">
          <StepIcon icon={KeyRoundIcon} />
          <div className="flex flex-1 flex-col gap-1">
            <CardTitle>Automate from your CI or editor</CardTitle>
            <CardDescription>
              Create a workspace API key to call the TestForge API, and later the CLI and MCP server.
            </CardDescription>
          </div>
          <Button variant="ghost" size="sm" asChild>
            <Link href={`/w/${workspace.slug}/settings/api-keys`}>
              API keys
              <ArrowRightIcon data-icon="inline-end" />
            </Link>
          </Button>
        </CardHeader>
      </Card>
    </PageContainer>
  );
}
