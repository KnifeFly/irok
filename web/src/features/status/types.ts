export type Status = {
  started_at: string;
  uptime_seconds: number;
  requests_total: number;
  failures_total: number;
  nodes_total: number;
  prompts_total: number;
  models: string[];
  config_warnings: string[];
  config: {
    server: {
      host: string;
      port: number;
      public_url: string;
    };
    files: {
      config_dir: string;
      pools_path: string;
      prompts_path: string;
      credentials_dir: string;
    };
    kiro: {
      default_model: string;
      default_region: string;
      assistant_identity: string;
    };
  };
};
