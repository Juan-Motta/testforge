"use client";

import {
  MutationCache,
  QueryCache,
  QueryClient,
  QueryClientProvider,
} from "@tanstack/react-query";
import { ThemeProvider } from "next-themes";
import { useState, type ReactNode } from "react";

import { Toaster } from "@/components/ui/sonner";
import { TooltipProvider } from "@/components/ui/tooltip";
import { isApiError } from "@/lib/api/errors";
import { shouldRedirectToLogin } from "@/lib/api/unauthorized";

function redirectOnUnauthorized(error: unknown, meta: Record<string, unknown> | undefined) {
  if (shouldRedirectToLogin(error, meta)) {
    // Outside React (query cache callback): a full navigation also drops all cached state.
    // eslint-disable-next-line @next/next/no-location-assign-relative-destination
    window.location.assign("/login");
  }
}

function makeQueryClient() {
  return new QueryClient({
    queryCache: new QueryCache({
      onError: (error, query) => redirectOnUnauthorized(error, query.meta),
    }),
    mutationCache: new MutationCache({
      onError: (error, _variables, _context, mutation) =>
        redirectOnUnauthorized(error, mutation.meta),
    }),
    defaultOptions: {
      queries: {
        staleTime: 30_000,
        retry: (failureCount, error) =>
          !(isApiError(error) && error.status >= 400 && error.status < 500) &&
          failureCount < 2,
      },
    },
  });
}

export function Providers({ children }: { children: ReactNode }) {
  const [queryClient] = useState(makeQueryClient);

  return (
    <ThemeProvider
      attribute="class"
      defaultTheme="system"
      enableSystem
      disableTransitionOnChange
    >
      <QueryClientProvider client={queryClient}>
        <TooltipProvider delayDuration={300}>
          {children}
          <Toaster position="bottom-right" />
        </TooltipProvider>
      </QueryClientProvider>
    </ThemeProvider>
  );
}
