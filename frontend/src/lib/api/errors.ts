import type { ApiErrorBody, ApiErrorCode } from "./types";

/** Error raised for any non-2xx API response, following the contract's `Error` shape. */
export class ApiError extends Error {
  readonly status: number;
  readonly code: ApiErrorCode | "network";
  readonly fields: Record<string, string>;

  constructor(
    status: number,
    code: ApiErrorCode | "network",
    message: string,
    fields: Record<string, string> = {},
  ) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.fields = fields;
  }
}

function isErrorBody(value: unknown): value is ApiErrorBody {
  if (typeof value !== "object" || value === null || !("error" in value)) {
    return false;
  }
  const error = (value as { error: unknown }).error;
  return (
    typeof error === "object" &&
    error !== null &&
    typeof (error as { code?: unknown }).code === "string" &&
    typeof (error as { message?: unknown }).message === "string"
  );
}

/** Builds an ApiError from a failed response and its (possibly missing) JSON body. */
export function toApiError(response: Response, body: unknown): ApiError {
  if (isErrorBody(body)) {
    return new ApiError(
      response.status,
      body.error.code,
      body.error.message,
      body.error.fields ?? {},
    );
  }
  const code: ApiErrorCode =
    response.status === 401
      ? "unauthorized"
      : response.status === 403
        ? "forbidden"
        : response.status === 404
          ? "not_found"
          : "internal";
  return new ApiError(
    response.status,
    code,
    response.statusText || "Unexpected server error",
  );
}

export function isApiError(error: unknown): error is ApiError {
  return error instanceof ApiError;
}

/** Human-friendly message for toasts and inline alerts. */
export function errorMessage(error: unknown): string {
  if (isApiError(error)) {
    if (error.code === "network") return error.message;
    if (error.code === "internal") {
      return "Something went wrong on our side. Please try again.";
    }
    return error.message;
  }
  return "Something went wrong. Please try again.";
}
