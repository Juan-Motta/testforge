/** DOM id of the input (or list message) for a form path, e.g. `sandbox-tools-0-name`. */
export function fieldId(path: string): string {
  return `sandbox-${path.replaceAll(".", "-")}`;
}
