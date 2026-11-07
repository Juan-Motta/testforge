import type { components } from "./schema";

type Schemas = components["schemas"];

export type ApiErrorBody = Schemas["Error"];
export type ApiErrorCode = ApiErrorBody["error"]["code"];
export type Role = Schemas["Role"];
export type User = Schemas["User"];
export type Workspace = Schemas["Workspace"];
export type Me = Schemas["Me"];
export type ApiKey = Schemas["ApiKey"];
export type CreatedApiKey = Schemas["CreatedApiKey"];
export type SignupRequest = Schemas["SignupRequest"];
export type LoginRequest = Schemas["LoginRequest"];
export type CreateWorkspaceRequest = Schemas["CreateWorkspaceRequest"];
export type CreateApiKeyRequest = Schemas["CreateApiKeyRequest"];
