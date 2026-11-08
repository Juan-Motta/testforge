/** DOM id of the input (or list message) for a form path, e.g. `scenario-criteria-0-weight`. */
export function fieldId(path: string): string {
  return `scenario-${path.replaceAll(".", "-")}`;
}
