import type { Role } from "@/lib/api/types";

/** Owners, admins and members launch, cancel and rerun runs; viewers only read. */
export function canOperateRuns(role: Role): boolean {
  return role !== "viewer";
}
