"use client";

import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import type { Capability, Connection } from "@/lib/api/types";

// Radix Select does not accept an empty value, so "not set" has its own token.
const NONE = "none";

type ConnectionSelectProps = {
  id: string;
  value: string;
  onChange: (connectionId: string) => void;
  connections: Connection[] | undefined;
  capability: Capability;
  invalid?: boolean;
  disabled?: boolean;
};

/** Picks a connection with the given capability; "" means not set. */
export function ConnectionSelect({ id, value, onChange, connections, capability, invalid, disabled }: ConnectionSelectProps) {
  const options = connections?.filter((connection) => connection.capabilities.includes(capability)) ?? [];
  return (
    <Select value={value === "" ? NONE : value} onValueChange={(next) => onChange(next === NONE ? "" : next)} disabled={disabled}>
      <SelectTrigger id={id} className="w-full" aria-invalid={invalid}>
        <SelectValue placeholder="Not set" />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value={NONE}>Not set</SelectItem>
        {options.map((connection) => (
          <SelectItem key={connection.id} value={connection.id}>
            {connection.name}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
