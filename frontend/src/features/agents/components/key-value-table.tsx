"use client";

import { PlusIcon, XIcon } from "lucide-react";
import { Controller, useFieldArray, useFormContext, type Path } from "react-hook-form";

import { Button } from "@/components/ui/button";
import { FieldError } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

import type { AgentFormValues } from "../lib/form";
import { fieldId } from "./field-id";
import { ListError } from "./list-error";

type RowListPath =
  | "transport.query"
  | "transport.headers"
  | "transport.preSession.headers"
  | "transport.preSession.captures"
  | "variables";

type Column = { field: string; label: string; placeholder?: string };

type KeyValueTableProps = {
  name: RowListPath;
  label: string;
  /** Defaults to key / value. */
  columns?: [Column, Column];
  addLabel?: string;
  emptyText?: string;
  readOnly?: boolean;
};

const KEY_VALUE: [Column, Column] = [
  { field: "key", label: "Key" },
  { field: "value", label: "Value" },
];

/** Editable list of two-column rows (query params, headers, variables, captures). */
export function KeyValueTable({
  name,
  label,
  columns = KEY_VALUE,
  addLabel = "Add row",
  emptyText = "None yet.",
  readOnly,
}: KeyValueTableProps) {
  const { control } = useFormContext<AgentFormValues>();
  const { fields, append, remove } = useFieldArray({ control, name: name as "variables" });

  return (
    <div className="flex flex-col gap-2" role="group" aria-label={label}>
      <div className="flex items-center justify-between gap-2">
        <span className="text-sm font-medium">{label}</span>
        {!readOnly && (
          <Button
            type="button"
            variant="ghost"
            size="xs"
            onClick={() => append({ [columns[0].field]: "", [columns[1].field]: "" } as never)}
          >
            <PlusIcon />
            {addLabel}
          </Button>
        )}
      </div>
      <ListError name={name} />
      {fields.length === 0 ? (
        <p className="rounded-lg border border-dashed px-3 py-2.5 text-xs text-muted-foreground">{emptyText}</p>
      ) : (
        <div className="flex flex-col gap-2">
          <div className="grid grid-cols-[minmax(0,2fr)_minmax(0,3fr)_1.75rem] gap-2 px-0.5 text-xs text-muted-foreground max-sm:hidden">
            <span>{columns[0].label}</span>
            <span>{columns[1].label}</span>
          </div>
          {fields.map((row, index) => (
            <div
              key={row.id}
              className="grid grid-cols-[minmax(0,1fr)_1.75rem] gap-2 sm:grid-cols-[minmax(0,2fr)_minmax(0,3fr)_1.75rem]"
            >
              {columns.map((column, columnIndex) => {
                const path = `${name}.${index}.${column.field}` as Path<AgentFormValues>;
                return (
                  <Controller
                    key={column.field}
                    name={path}
                    control={control}
                    render={({ field, fieldState }) => (
                      <div className={cn("flex min-w-0 flex-col gap-1", columnIndex === 1 && "max-sm:col-start-1")}>
                        <Input
                          {...field}
                          value={(field.value as string | undefined) ?? ""}
                          id={fieldId(path)}
                          aria-label={`${label} ${index + 1} ${column.label.toLowerCase()}`}
                          aria-invalid={fieldState.invalid}
                          placeholder={column.placeholder ?? column.label}
                          autoComplete="off"
                          spellCheck={false}
                          className="font-mono text-xs md:text-xs"
                        />
                        {fieldState.error && <FieldError errors={[fieldState.error]} />}
                      </div>
                    )}
                  />
                );
              })}
              {!readOnly && (
                <Button
                  type="button"
                  variant="ghost"
                  size="icon-sm"
                  className="col-start-2 row-start-1 sm:col-start-3"
                  aria-label={`Remove ${label.toLowerCase()} row ${index + 1}`}
                  onClick={() => remove(index)}
                >
                  <XIcon />
                </Button>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
