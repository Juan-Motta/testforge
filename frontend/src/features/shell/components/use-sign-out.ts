"use client";

import { useRouter } from "next/navigation";
import { toast } from "sonner";

import { useLogout } from "@/features/auth/api/auth";
import { errorMessage } from "@/lib/api/errors";

export function useSignOut() {
  const router = useRouter();
  const logout = useLogout();
  return () =>
    logout.mutate(undefined, {
      onSuccess: () => {
        router.replace("/login");
        router.refresh();
      },
      onError: (error) => toast.error(errorMessage(error)),
    });
}
