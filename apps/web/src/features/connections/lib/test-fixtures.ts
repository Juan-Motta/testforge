import type { Connection, ConnectionKind } from "@/lib/api/types";

export const azureKind: ConnectionKind = {
  kind: "azure_openai",
  label: "Azure OpenAI",
  description: "Models deployed in your Azure OpenAI resource.",
  capabilities: ["llm", "realtime"],
  secretLabel: "API key",
  docsUrl: "https://learn.microsoft.com/azure/ai-services/openai/",
  fields: [
    {
      key: "endpoint",
      label: "Endpoint",
      type: "url",
      required: true,
      placeholder: "https://my-resource.openai.azure.com",
    },
    {
      key: "apiVersion",
      label: "API version",
      type: "text",
      required: true,
      default: "2024-10-21",
      pattern: "^\\d{4}-\\d{2}-\\d{2}(-preview)?$",
    },
    { key: "region", label: "Region", type: "text", required: false },
  ],
};

export const connection: Connection = {
  id: "7b0c4a9e-2f1d-4d8e-9a51-1d2b3c4d5e6f",
  name: "Azure prod",
  kind: "azure_openai",
  config: { endpoint: "https://acme.openai.azure.com", apiVersion: "2024-06-01" },
  secretHint: "•••• 9f2c",
  capabilities: ["llm", "realtime"],
  lastTest: null,
  createdBy: { id: "1b0c4a9e-2f1d-4d8e-9a51-1d2b3c4d5e6f", name: "Ada" },
  createdAt: "2026-11-01T10:00:00Z",
  updatedAt: "2026-11-01T10:00:00Z",
};
