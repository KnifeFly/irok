export type KiroNode = {
  id: string;
  name: string;
  credential_path: string;
  enabled: boolean;
  healthy: boolean;
  failure_count: number;
  last_error?: string;
  last_error_at?: string;
  recovery_at?: string;
  last_used_at?: string;
  usage_count: number;
  note?: string;
  created_at?: string;
  updated_at?: string;
};
