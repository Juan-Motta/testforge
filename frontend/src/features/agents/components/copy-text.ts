import { toast } from "sonner";

/** Copies a value and confirms it with a toast. */
export async function copyText(value: string, what = "Copied to clipboard"): Promise<void> {
  try {
    await navigator.clipboard.writeText(value);
    toast.success(what);
  } catch {
    toast.error("Copy failed. Select the text and copy it manually.");
  }
}
