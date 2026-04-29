import { apiClient } from "@/shared/api/api-client";
import type { Status } from "@/features/status/types";

export function getStatus() {
  return apiClient.get<Status>("/api/status");
}

export function getLogTail(limit = 160) {
  return apiClient.get<{ lines: string[] }>(`/api/logs/tail?limit=${limit}`);
}

export async function validateAdminKey(adminKey: string) {
  const response = await fetch("/api/status", {
    headers: {
      Authorization: `Bearer ${adminKey}`,
      "Content-Type": "application/json",
    },
  });
  let body: unknown = null;
  try {
    body = await response.json();
  } catch {
    body = null;
  }
  if (!response.ok) {
    throw new Error(response.status === 401 ? "管理员密码不正确" : `登录失败：HTTP ${response.status}`);
  }
  if (body && typeof body === "object" && "success" in body) {
    const wrapped = body as { success: boolean; data?: Status; message?: string };
    if (!wrapped.success || !wrapped.data) {
      throw new Error(wrapped.message ?? "登录失败");
    }
    return wrapped.data;
  }
  return body as Status;
}
