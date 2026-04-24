import { apiClient } from "@/shared/api/api-client";
import type { Status } from "@/features/status/types";

export function getStatus() {
  return apiClient.get<Status>("/api/status");
}

export function getLogTail(limit = 160) {
  return apiClient.get<{ lines: string[] }>(`/api/logs/tail?limit=${limit}`);
}
