import { useSessionStore } from "@/shared/api/session-store";

export type ApiResponse<T> = {
  success: boolean;
  data: T;
  message?: string;
};

export class ApiError extends Error {
  status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

function headers(extra?: HeadersInit): Headers {
  const out = new Headers(extra);
  const key = useSessionStore.getState().adminKey;
  if (key) {
    out.set("Authorization", `Bearer ${key}`);
  }
  if (!out.has("Content-Type")) {
    out.set("Content-Type", "application/json");
  }
  return out;
}

async function parse<T>(response: Response): Promise<T> {
  let body: unknown = null;
  try {
    body = await response.json();
  } catch {
    if (!response.ok) {
      throw new ApiError(`HTTP ${response.status}`, response.status);
    }
  }
  if (!response.ok) {
    const message =
      body && typeof body === "object" && "message" in body && typeof body.message === "string"
        ? body.message
        : `HTTP ${response.status}`;
    throw new ApiError(message, response.status);
  }
  if (body && typeof body === "object" && "success" in body) {
    const wrapped = body as ApiResponse<T>;
    if (!wrapped.success) {
      throw new ApiError(wrapped.message ?? "Request failed", response.status);
    }
    return wrapped.data;
  }
  return body as T;
}

export const apiClient = {
  async get<T>(path: string): Promise<T> {
    const response = await fetch(path, { headers: headers() });
    return parse<T>(response);
  },
  async post<T>(path: string, body?: unknown): Promise<T> {
    const response = await fetch(path, {
      method: "POST",
      headers: headers(),
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    return parse<T>(response);
  },
  async put<T>(path: string, body?: unknown): Promise<T> {
    const response = await fetch(path, {
      method: "PUT",
      headers: headers(),
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    return parse<T>(response);
  },
  async delete<T>(path: string): Promise<T> {
    const response = await fetch(path, { method: "DELETE", headers: headers() });
    return parse<T>(response);
  },
};
