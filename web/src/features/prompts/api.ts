import { apiClient } from "@/shared/api/api-client";
import type { PromptRule } from "@/features/prompts/types";

export function listPrompts() {
  return apiClient.get<PromptRule[]>("/api/prompts");
}

export function savePrompts(prompts: PromptRule[]) {
  return apiClient.put<PromptRule[]>("/api/prompts", { prompts });
}
