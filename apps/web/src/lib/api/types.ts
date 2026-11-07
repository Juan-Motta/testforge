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
export type Capability = Schemas["Capability"];
export type ConnectionKindId = Schemas["ConnectionKindId"];
export type ConnectionField = Schemas["ConnectionField"];
export type ConnectionKind = Schemas["ConnectionKind"];
export type ConnectionTest = Schemas["ConnectionTest"];
export type Connection = Schemas["Connection"];
export type CreateConnectionRequest = Schemas["CreateConnectionRequest"];
export type UpdateConnectionRequest = Schemas["UpdateConnectionRequest"];
