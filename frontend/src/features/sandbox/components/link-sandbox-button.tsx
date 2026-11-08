"use client";

import { LinkIcon } from "lucide-react";
import { useRouter } from "next/navigation";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { sectionHref, SECTIONS } from "@/features/shell/lib/navigation";
import { errorMessage } from "@/lib/api/errors";

import { useLinkSandboxAgent } from "../api/sandbox";

type LinkSandboxButtonProps = {
  slug: string;
  agentId: string;
  /** Owners and admins only, since linking creates an API key. */
  canLink: boolean;
};

/** "Use in scenarios": creates a regular agent that calls this sandbox agent and opens it. */
export function LinkSandboxButton({ slug, agentId, canLink }: LinkSandboxButtonProps) {
  const router = useRouter();
  const link = useLinkSandboxAgent(slug);

  function run() {
    link.mutate(agentId, {
      onSuccess: (agent) => {
        toast.success(`Agent “${agent.name}” created`, {
          description: "It calls this sandbox agent with a new workspace API key. Use it in scenarios and runs.",
        });
        router.push(`${sectionHref(slug, SECTIONS.agents)}/${agent.id}`);
      },
      onError: (error) => toast.error(errorMessage(error)),
    });
  }

  const button = (
    <Button type="button" variant="outline" onClick={run} disabled={!canLink || link.isPending}>
      {link.isPending ? <Spinner /> : <LinkIcon />}
      <span className="max-sm:sr-only">Use in scenarios</span>
    </Button>
  );
  if (canLink) return button;
  return (
    <Tooltip>
      {/* Disabled buttons do not fire pointer events, so the wrapper carries the tooltip. */}
      <TooltipTrigger asChild>
        <span tabIndex={0} className="rounded-md outline-none focus-visible:ring-3 focus-visible:ring-ring/50">
          {button}
        </span>
      </TooltipTrigger>
      <TooltipContent>Only owners and admins can link sandbox agents, since it creates an API key.</TooltipContent>
    </Tooltip>
  );
}
