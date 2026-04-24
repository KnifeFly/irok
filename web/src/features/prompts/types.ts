export type PromptRule = {
  model: string;
  enabled: boolean;
  mode: "prepend" | "append" | "override" | "off";
  content: string;
  note?: string;
  updated_at?: string;
};
