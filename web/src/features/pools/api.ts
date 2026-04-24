import { apiClient } from "@/shared/api/api-client";
import type { KiroNode } from "@/features/pools/types";

export function listNodes() {
  return apiClient.get<KiroNode[]>("/api/pools/kiro/nodes");
}

export function saveNode(node: Partial<KiroNode>) {
  if (node.id) {
    return apiClient.put<KiroNode>(`/api/pools/kiro/nodes/${node.id}`, node);
  }
  return apiClient.post<KiroNode>("/api/pools/kiro/nodes", node);
}

export function deleteNode(id: string) {
  return apiClient.delete<{ deleted: boolean }>(`/api/pools/kiro/nodes/${id}`);
}

export function refreshNode(id: string) {
  return apiClient.post<{ refreshed: boolean }>(`/api/pools/kiro/nodes/${id}/refresh`);
}

export function startOAuth(body: { method: string; node_name?: string; region?: string }) {
  return apiClient.post<{ auth_url: string; auth_info: Record<string, unknown> }>("/api/kiro/oauth/start", body);
}

export function importCredentials(body: { name?: string; credentials: unknown }) {
  return apiClient.post<KiroNode[]>("/api/kiro/credentials/import", body);
}
