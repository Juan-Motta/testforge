"use client";

import { CircleCheckBigIcon, RotateCwIcon, ZapIcon } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";

import { FormError } from "@/components/form-error";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Skeleton } from "@/components/ui/skeleton";
import { Spinner } from "@/components/ui/spinner";
import { errorMessage } from "@/lib/api/errors";
import type { Connection, ConnectionKind, ConnectionTest } from "@/lib/api/types";

import { useConnectionKinds, useCreateConnection, useUpdateConnection } from "../api/connections";
import { buildCreateRequest, buildUpdateRequest } from "../lib/form";
import { KindIcon } from "./capability-badges";
import { ConnectionForm } from "./connection-form";
import { KindPicker } from "./kind-picker";
import { LastTestStatus } from "./last-test-status";
import { useConnectionTester } from "./use-connection-tester";

type ConnectionDialogProps = {
  slug: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** The connection to edit; omit to add a new one. */
  connection?: Connection | null;
};

function CatalogBody({ children }: { children: (kinds: ConnectionKind[]) => React.ReactNode }) {
  const kinds = useConnectionKinds();
  if (kinds.isError) {
    return (
      <div className="flex flex-col items-start gap-3">
        <FormError message={`Could not load providers. ${errorMessage(kinds.error)}`} />
        <Button variant="outline" size="sm" onClick={() => kinds.refetch()}>
          <RotateCwIcon />
          Try again
        </Button>
      </div>
    );
  }
  if (!kinds.data) {
    return (
      <div className="grid gap-3 sm:grid-cols-2" aria-busy>
        {Array.from({ length: 4 }, (_, index) => (
          <Skeleton key={index} className="h-32 rounded-xl" />
        ))}
      </div>
    );
  }
  return children(kinds.data);
}

export function ConnectionDialog({ slug, open, onOpenChange, connection }: ConnectionDialogProps) {
  const create = useCreateConnection(slug);
  const update = useUpdateConnection(slug);
  const tester = useConnectionTester(slug);
  const [selected, setSelected] = useState<ConnectionKind | null>(null);
  const [created, setCreated] = useState<Connection | null>(null);
  const [testResult, setTestResult] = useState<ConnectionTest | null>(null);

  // Keep the last edited connection so the content does not flash during the close animation.
  const [lastEdited, setLastEdited] = useState(connection ?? null);
  if (connection && connection !== lastEdited) setLastEdited(connection);
  const editing = open ? (connection ?? null) : lastEdited;

  function resetDialog() {
    setSelected(null);
    setCreated(null);
    setTestResult(null);
    setLastEdited(null);
  }

  async function runTest(target: Connection) {
    const result = await tester.run(target);
    if (result) setTestResult(result);
  }

  let title: string;
  let description: string;
  let body: React.ReactNode;

  if (created) {
    title = "Connection added";
    description = `“${created.name}” is ready. Check that the credentials work before you use it.`;
    const testing = tester.isTesting(created.id);
    body = (
      <div className="flex flex-col gap-6">
        <div className="flex items-center gap-3 rounded-xl border bg-muted/30 p-4">
          <CircleCheckBigIcon aria-hidden className="size-5 shrink-0 text-success" />
          <div className="flex min-w-0 flex-col gap-0.5">
            <p className="truncate text-sm font-medium">{created.name}</p>
            <LastTestStatus lastTest={testResult} testing={testing} />
          </div>
        </div>
        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            Done
          </Button>
          <Button type="button" onClick={() => runTest(created)} disabled={testing}>
            {testing ? <Spinner /> : <ZapIcon />}
            {testResult ? "Test again" : "Test connection"}
          </Button>
        </DialogFooter>
      </div>
    );
  } else if (editing) {
    title = `Edit “${editing.name}”`;
    description = "Changing the settings or the secret clears the last test result.";
    body = (
      <CatalogBody>
        {(kinds) => {
          const kind = kinds.find((k) => k.kind === editing.kind);
          if (!kind) return <FormError message="This provider is no longer supported." />;
          return (
            <ConnectionForm
              key={editing.id}
              kind={kind}
              mode="edit"
              connection={editing}
              submitLabel="Save changes"
              onSubmit={(values) =>
                update
                  .mutateAsync({ id: editing.id, body: buildUpdateRequest(kind, values) })
                  .then((saved) => {
                    toast.success(`Connection “${saved.name}” updated`);
                    onOpenChange(false);
                  })
              }
            />
          );
        }}
      </CatalogBody>
    );
  } else if (selected) {
    title = `Connect ${selected.label}`;
    description = selected.description;
    body = (
      <ConnectionForm
        key={selected.kind}
        kind={selected}
        mode="create"
        submitLabel="Add connection"
        onBack={() => setSelected(null)}
        onSubmit={(values) =>
          create.mutateAsync(buildCreateRequest(selected, values)).then(setCreated)
        }
      />
    );
  } else {
    title = "Add connection";
    description = "Pick the provider you want TestForge to use.";
    body = <CatalogBody>{(kinds) => <KindPicker kinds={kinds} onSelect={setSelected} />}</CatalogBody>;
  }

  const headerKind = selected ?? null;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className="max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-xl"
        onCloseAutoFocus={resetDialog}
      >
        <div className="flex flex-col gap-6">
          <DialogHeader className="flex-row items-start gap-3 text-left">
            {headerKind && !created && <KindIcon capabilities={headerKind.capabilities} />}
            <div className="flex flex-col gap-1.5">
              <DialogTitle>{title}</DialogTitle>
              <DialogDescription>{description}</DialogDescription>
            </div>
          </DialogHeader>
          {body}
        </div>
      </DialogContent>
    </Dialog>
  );
}
