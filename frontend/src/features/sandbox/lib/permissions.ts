import type { Role } from "@/lib/api/types";

/** Owners, admins and members write sandbox agents, chat, generate drafts and delete memories; viewers only read. */
export function canEditSandbox(role: Role): boolean {
  return role !== "viewer";
}

/** Linking creates an API key, so only owners and admins can do it (like API keys). */
export function canLinkSandbox(role: Role): boolean {
  return role === "owner" || role === "admin";
}
