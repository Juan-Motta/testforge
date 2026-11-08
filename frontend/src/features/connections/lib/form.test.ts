import { describe, expect, it } from "vitest";

import { azureKind, connection } from "./test-fixtures";
import {
  buildConnectionSchema,
  buildCreateRequest,
  buildUpdateRequest,
  initialFormValues,
  mapServerFieldErrors,
} from "./form";

describe("initialFormValues", () => {
  it("prefills the kind label as name and field defaults on create", () => {
    expect(initialFormValues(azureKind)).toEqual({
      name: "Azure OpenAI",
      secret: "",
      config: { endpoint: "", apiVersion: "2024-10-21", region: "" },
    });
  });

  it("uses the stored name and config on edit, never a secret", () => {
    expect(initialFormValues(azureKind, connection)).toEqual({
      name: "Azure prod",
      secret: "",
      config: { endpoint: "https://acme.openai.azure.com", apiVersion: "2024-06-01", region: "" },
    });
  });
});

describe("buildCreateRequest", () => {
  it("trims values, applies defaults to empty fields and omits empty optional ones", () => {
    expect(
      buildCreateRequest(azureKind, {
        name: "  Azure  ",
        secret: " sk-123 ",
        config: { endpoint: " https://x.openai.azure.com ", apiVersion: "", region: "" },
      }),
    ).toEqual({
      name: "Azure",
      kind: "azure_openai",
      secret: "sk-123",
      config: { endpoint: "https://x.openai.azure.com", apiVersion: "2024-10-21" },
    });
  });
});

describe("buildUpdateRequest", () => {
  it("omits an empty secret so the stored one is kept", () => {
    const body = buildUpdateRequest(azureKind, {
      name: "Azure prod",
      secret: "   ",
      config: { endpoint: "https://acme.openai.azure.com", apiVersion: "2024-06-01", region: "westeurope" },
    });
    expect(body).toEqual({
      name: "Azure prod",
      config: {
        endpoint: "https://acme.openai.azure.com",
        apiVersion: "2024-06-01",
        region: "westeurope",
      },
    });
    expect(body).not.toHaveProperty("secret");
  });

  it("sends a new secret when one is entered", () => {
    expect(
      buildUpdateRequest(azureKind, { name: "A", secret: "new", config: { endpoint: "https://a.b" } })
        .secret,
    ).toBe("new");
  });
});

describe("buildConnectionSchema", () => {
  const valid = {
    name: "Azure",
    secret: "sk",
    config: { endpoint: "https://x.openai.azure.com", apiVersion: "2024-10-21", region: "" },
  };

  it("accepts valid values", () => {
    expect(buildConnectionSchema(azureKind, "create").safeParse(valid).success).toBe(true);
  });

  function issues(mode: "create" | "edit", values: unknown) {
    const result = buildConnectionSchema(azureKind, mode).safeParse(values);
    return result.success ? {} : Object.fromEntries(result.error.issues.map((i) => [i.path.join("."), i.message]));
  }

  it("requires the secret on create but not on edit", () => {
    expect(issues("create", { ...valid, secret: "" })).toEqual({ secret: "Enter the API key." });
    expect(issues("edit", { ...valid, secret: "" })).toEqual({});
  });

  it("validates required fields, URLs and patterns", () => {
    expect(
      issues("create", { ...valid, config: { endpoint: "", apiVersion: "v1", region: "" } }),
    ).toEqual({
      "config.endpoint": "Enter the endpoint.",
      "config.apiVersion": "API version has an invalid format.",
    });
    expect(issues("create", { ...valid, config: { ...valid.config, endpoint: "not a url" } })).toEqual({
      "config.endpoint": "Enter a valid URL, e.g. https://my-resource.openai.azure.com.",
    });
  });
});

describe("mapServerFieldErrors", () => {
  it("maps name, secret and config.<key> onto form fields and reports the rest", () => {
    expect(
      mapServerFieldErrors(azureKind, {
        name: "Name is taken.",
        secret: "Too long.",
        "config.endpoint": "Must use https.",
        "config.unknown": "Nope.",
        kind: "Invalid kind.",
      }),
    ).toEqual({
      matched: [
        ["name", "Name is taken."],
        ["secret", "Too long."],
        ["config.endpoint", "Must use https."],
      ],
      unmatched: ["Nope.", "Invalid kind."],
    });
  });
});
