import {
  ActivityIcon,
  AudioLinesIcon,
  ClipboardCheckIcon,
  type LucideIcon,
} from "lucide-react";
import Link from "next/link";
import type { ReactNode } from "react";

import { Logo } from "@/components/logo";

const HIGHLIGHTS: { icon: LucideIcon; title: string; description: string }[] = [
  {
    icon: AudioLinesIcon,
    title: "Test voice and text agents",
    description: "Connect any agent over WebSocket or HTTP and talk to it like a real user would.",
  },
  {
    icon: ClipboardCheckIcon,
    title: "Scenarios with clear criteria",
    description: "Simulated personas follow your scenarios so every release faces the same conversations.",
  },
  {
    icon: ActivityIcon,
    title: "Traces and evaluations",
    description: "Pull Langfuse traces, score every conversation and spot regressions before your users do.",
  },
];

type AuthShellProps = {
  title: string;
  description: string;
  children: ReactNode;
};

export function AuthShell({ title, description, children }: AuthShellProps) {
  return (
    <div className="grid min-h-svh lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
      <div className="flex flex-col px-4 py-8 sm:px-8">
        <Link href="/" className="w-fit rounded-lg focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none">
          <Logo />
        </Link>
        <div className="flex flex-1 items-center justify-center py-12">
          <div className="flex w-full max-w-sm flex-col gap-8">
            <div className="flex flex-col gap-2">
              <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
              <p className="text-sm text-muted-foreground">{description}</p>
            </div>
            {children}
          </div>
        </div>
      </div>
      <aside className="relative hidden overflow-hidden border-l bg-muted/40 lg:flex lg:flex-col lg:justify-center lg:px-16">
        <div
          aria-hidden
          className="absolute inset-0 bg-[linear-gradient(to_right,var(--border)_1px,transparent_1px),linear-gradient(to_bottom,var(--border)_1px,transparent_1px)] mask-[radial-gradient(ellipse_at_center,black_20%,transparent_75%)] bg-size-[32px_32px]"
        />
        <div
          aria-hidden
          className="absolute -top-32 -right-32 size-96 rounded-full bg-brand/20 blur-3xl"
        />
        <div className="relative flex max-w-md flex-col gap-10">
          <div className="flex flex-col gap-3">
            <p className="text-sm font-medium text-brand">TestForge</p>
            <h2 className="text-3xl font-semibold tracking-tight text-balance">
              Ship AI agents you can trust in every conversation.
            </h2>
          </div>
          <ul className="flex flex-col gap-6">
            {HIGHLIGHTS.map(({ icon: Icon, title, description }) => (
              <li key={title} className="flex gap-4">
                <span className="flex size-9 shrink-0 items-center justify-center rounded-lg border bg-background shadow-xs">
                  <Icon className="size-4" />
                </span>
                <div className="flex flex-col gap-1">
                  <p className="text-sm font-medium">{title}</p>
                  <p className="text-sm text-muted-foreground">{description}</p>
                </div>
              </li>
            ))}
          </ul>
        </div>
      </aside>
    </div>
  );
}
