"use client";

import { useEffect } from "react";

/** Warns before closing or reloading the tab with unsaved changes. */
export function useBeforeUnload(active: boolean) {
  useEffect(() => {
    if (!active) return;
    const handler = (event: BeforeUnloadEvent) => event.preventDefault();
    window.addEventListener("beforeunload", handler);
    return () => window.removeEventListener("beforeunload", handler);
  }, [active]);
}
