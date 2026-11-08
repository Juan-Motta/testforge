import type { Role } from "@/lib/api/types";

/** Owners, admins and members write personas, groups and scenarios; viewers only read. */
export function canAuthorScenarios(role: Role): boolean {
  return role !== "viewer";
}
