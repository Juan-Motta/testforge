"use client";

import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import type { Connection } from "@/lib/api/types";

type ConnectionSelectProps = {
  id: string;
  value: string;
  onChange: (connectionId: string) => void;
  /** Connections already filtered to the allowed ones. */
  options: Connection[];
  placeholder?: string;
  invalid?: boolean;
  disabled?: boolean;
};

/** Picks one of the given connections; "" means none is picked yet. */
export function ConnectionSelect({ id, value, onChange, options, placeholder, invalid, disabled }: ConnectionSelectProps) {
  return (
    <Select value={value} onValueChange={onChange} disabled={disabled}>
      <SelectTrigger id={id} className="w-full" aria-invalid={invalid}>
        <SelectValue placeholder={placeholder ?? "Choose a connection"} />
      </SelectTrigger>
      <SelectContent>
        {options.length === 0 && (
          <div className="px-2 py-1.5 text-sm text-muted-foreground">No matching connections.</div>
        )}
        {options.map((connection) => (
          <SelectItem key={connection.id} value={connection.id}>
            {connection.name}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
