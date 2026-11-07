/** DOM id of the input (or list message) for a form path, e.g. `agent-transport-url`. */
export function fieldId(path: string): string {
  return `agent-${path.replaceAll(".", "-")}`;
}
