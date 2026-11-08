import {
  BarChart3Icon,
  BotIcon,
  ClipboardListIcon,
  FlaskConicalIcon,
  HouseIcon,
  PlayIcon,
  PlugIcon,
  SettingsIcon,
  UsersRoundIcon,
  type LucideIcon,
} from "lucide-react";

export type SectionId =
  | "overview"
  | "agents"
  | "scenarios"
  | "personas"
  | "sandbox"
  | "runs"
  | "reports"
  | "connections"
  | "settings";

export type Section = {
  id: SectionId;
  title: string;
  /** Path segment under `/w/[slug]`; empty for the overview. */
  segment: string;
  icon: LucideIcon;
  /** One line describing what the section is for. */
  description: string;
  /** Empty-state title shown until the section ships. */
  emptyTitle?: string;
  keywords?: string[];
};

export const SECTIONS: Record<SectionId, Section> = {
  overview: {
    id: "overview",
    title: "Overview",
    segment: "",
    icon: HouseIcon,
    description: "Get started and see what is happening in this workspace.",
    keywords: ["home", "dashboard", "getting started"],
  },
  agents: {
    id: "agents",
    title: "Agents",
    segment: "agents",
    icon: BotIcon,
    description:
      "Connect the voice and text agents you want to test over WebSocket or HTTP, with any headers, query params or body they need.",
    emptyTitle: "No agents yet",
    keywords: ["targets", "websocket", "http", "endpoints"],
  },
  scenarios: {
    id: "scenarios",
    title: "Scenarios",
    segment: "scenarios",
    icon: ClipboardListIcon,
    description:
      "Describe the conversations to test: the goal, the context and the criteria a good answer must meet.",
    emptyTitle: "No scenarios yet",
    keywords: ["tests", "cases", "criteria"],
  },
  personas: {
    id: "personas",
    title: "Personas",
    segment: "personas",
    icon: UsersRoundIcon,
    description:
      "Define the simulated users that talk to your agent: their tone, background, voice and patience.",
    emptyTitle: "No personas yet",
    keywords: ["simulated users", "callers", "voices"],
  },
  sandbox: {
    id: "sandbox",
    title: "Sandbox",
    segment: "sandbox",
    icon: FlaskConicalIcon,
    description:
      "Build hosted test agents with a system prompt, memory and mock tools, chat with them and test them with scenarios.",
    emptyTitle: "No sandbox agents yet",
    keywords: ["playground", "mock tools", "test agents", "memory"],
  },
  runs: {
    id: "runs",
    title: "Runs",
    segment: "runs",
    icon: PlayIcon,
    description:
      "Launch scenarios against an agent and follow each conversation live, with transcripts and audio.",
    emptyTitle: "No runs yet",
    keywords: ["executions", "jobs", "live"],
  },
  reports: {
    id: "reports",
    title: "Reports",
    segment: "reports",
    icon: BarChart3Icon,
    description:
      "Review evaluation results, Langfuse traces and trends across runs to see where your agent falls short.",
    emptyTitle: "No reports yet",
    keywords: ["evaluations", "results", "traces", "langfuse"],
  },
  connections: {
    id: "connections",
    title: "Connections",
    segment: "connections",
    icon: PlugIcon,
    description:
      "Store credentials for LLM, speech and tracing providers such as OpenAI, ElevenLabs and Langfuse, encrypted at rest.",
    emptyTitle: "No connections yet",
    keywords: ["providers", "credentials", "langfuse", "openai"],
  },
  settings: {
    id: "settings",
    title: "Settings",
    segment: "settings",
    icon: SettingsIcon,
    description: "Manage this workspace and its API keys.",
    keywords: ["preferences", "workspace", "api keys"],
  },
};

export type NavGroup = { label: string; items: Section[] };

export const NAV_GROUPS: NavGroup[] = [
  { label: "Build", items: [SECTIONS.agents, SECTIONS.sandbox, SECTIONS.scenarios, SECTIONS.personas] },
  { label: "Test", items: [SECTIONS.runs, SECTIONS.reports] },
  { label: "Configure", items: [SECTIONS.connections, SECTIONS.settings] },
];

export function sectionHref(slug: string, section: Section | string): string {
  const segment = typeof section === "string" ? section : section.segment;
  return segment ? `/w/${slug}/${segment}` : `/w/${slug}`;
}

/** Sub-pages that appear in breadcrumbs, keyed by their path segment. */
export const SUBPAGE_TITLES: Record<string, string> = {
  "api-keys": "API keys",
};

/** Finds the active section from a pathname such as `/w/acme/settings/api-keys`. */
export function sectionFromPathname(pathname: string): Section {
  const segment = pathname.split("/")[3] ?? "";
  return (
    Object.values(SECTIONS).find((section) => section.segment === segment) ??
    SECTIONS.overview
  );
}
