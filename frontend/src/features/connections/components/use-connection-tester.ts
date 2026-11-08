"use client";

import { useState } from "react";
import { toast } from "sonner";

import { errorMessage } from "@/lib/api/errors";
import type { Connection, ConnectionTest } from "@/lib/api/types";

import { useTestConnection } from "../api/connections";

/** Runs connection tests (several may run at once), toasting each outcome. */
export function useConnectionTester(slug: string) {
  const testConnection = useTestConnection(slug);
  const [testing, setTesting] = useState<ReadonlySet<string>>(new Set());

  function update(id: string, active: boolean) {
    setTesting((current) => {
      const next = new Set(current);
      if (active) next.add(id);
      else next.delete(id);
      return next;
    });
  }

  async function run(connection: Pick<Connection, "id" | "name">): Promise<ConnectionTest | null> {
    update(connection.id, true);
    try {
      const result = await testConnection.mutateAsync(connection.id);
      if (result.ok) {
        toast.success(`“${connection.name}” is connected`, {
          description: `${result.message} Responded in ${result.latencyMs} ms.`,
        });
      } else {
        toast.error(`“${connection.name}” failed the test`, { description: result.message });
      }
      return result;
    } catch (error) {
      toast.error(errorMessage(error));
      return null;
    } finally {
      update(connection.id, false);
    }
  }

  return { run, isTesting: (id: string) => testing.has(id) };
}
