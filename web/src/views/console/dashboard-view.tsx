import { FormEvent, useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  AlertTriangleIcon,
  CheckCircle2Icon,
  ClockIcon,
  CopyIcon,
  ExternalLinkIcon,
  KeyRoundIcon,
  Loader2Icon,
  LogOutIcon,
  PlusIcon,
  RefreshCwIcon,
  ShieldCheckIcon,
  Trash2Icon,
  XCircleIcon,
} from "lucide-react";
import { toast } from "sonner";
import { z } from "zod";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle } from "@/components/ui/sheet";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Textarea } from "@/components/ui/textarea";
import { importCredentials, listNodes, refreshNode, saveNode, startOAuth, deleteNode } from "@/features/pools/api";
import type { CredentialStatus, KiroNode } from "@/features/pools/types";
import { listPrompts, savePrompts } from "@/features/prompts/api";
import type { PromptRule } from "@/features/prompts/types";
import { getLogTail, getStatus, validateAdminKey } from "@/features/status/api";
import { ApiError } from "@/shared/api/api-client";
import { PageShell, type NavSection } from "@/shared/layout/page-shell";
import { useSessionStore } from "@/shared/api/session-store";

const nodeSchema = z.object({
  name: z.string().min(1),
  credential_path: z.string().min(1),
  note: z.string().optional(),
});

const promptModes: PromptRule["mode"][] = ["prepend", "append", "override", "off"];
const promptModeLabels: Record<PromptRule["mode"], string> = {
  prepend: "前置",
  append: "追加",
  override: "覆盖",
  off: "关闭",
};

const sectionIds: NavSection[] = ["overview", "nodes", "prompts", "access", "logs"];
const sectionMeta: Record<NavSection, { title: string; description: string }> = {
  overview: {
    title: "orik 控制台",
    description: "管理 Kiro 账号池、模型系统提示词、OAuth 接入、日志和 Claude 兼容 API 状态。",
  },
  nodes: {
    title: "Kiro 账号池",
    description: "维护可用账号节点、凭据文件和刷新状态。",
  },
  prompts: {
    title: "模型系统提示词",
    description: "按模型配置系统提示词规则，精确模型匹配优先于通配规则。",
  },
  access: {
    title: "访问配置",
    description: "查看 Claude 兼容接口的调用方式和当前登录状态。",
  },
  logs: {
    title: "运行日志",
    description: "查看服务日志尾部内容，辅助排查鉴权、刷新和请求错误。",
  },
};

export function DashboardView() {
  const queryClient = useQueryClient();
  const { adminKey, clearAdminKey, rememberAdminKey, setAdminKey } = useSessionStore();
  const activeSection = useActiveSection();
  const currentSection = sectionMeta[activeSection];
  const [loginPassword, setLoginPassword] = useState("");
  const [rememberLogin, setRememberLogin] = useState(rememberAdminKey);
  const [nodeSheetOpen, setNodeSheetOpen] = useState(false);
  const [editingNode, setEditingNode] = useState<KiroNode | null>(null);
  const [credentialDialogOpen, setCredentialDialogOpen] = useState(false);
  const [oauthDialogOpen, setOauthDialogOpen] = useState(false);
  const [promptSheetOpen, setPromptSheetOpen] = useState(false);
  const [editingPrompt, setEditingPrompt] = useState<PromptRule | null>(null);

  const statusQuery = useQuery({ queryKey: ["status"], queryFn: getStatus, enabled: Boolean(adminKey) });
  const nodesQuery = useQuery({ queryKey: ["nodes"], queryFn: listNodes, enabled: Boolean(adminKey) });
  const promptsQuery = useQuery({ queryKey: ["prompts"], queryFn: listPrompts, enabled: Boolean(adminKey) });
  const logsQuery = useQuery({ queryKey: ["logs"], queryFn: () => getLogTail(160), enabled: Boolean(adminKey), refetchInterval: 15_000 });

  const loginMutation = useMutation({
    mutationFn: ({ password }: { password: string; remember: boolean }) => validateAdminKey(password),
    onSuccess: async (_status, variables) => {
      setAdminKey(variables.password, { remember: variables.remember });
      setLoginPassword("");
      toast.success("登录成功");
      await queryClient.invalidateQueries();
    },
    onError: (error) => toast.error(error.message),
  });

  const saveNodeMutation = useMutation({
    mutationFn: saveNode,
    onSuccess: async () => {
      toast.success("Kiro 节点已保存");
      setNodeSheetOpen(false);
      setEditingNode(null);
      await queryClient.invalidateQueries({ queryKey: ["nodes"] });
      await queryClient.invalidateQueries({ queryKey: ["status"] });
    },
    onError: (error) => toast.error(error.message),
  });

  const deleteNodeMutation = useMutation({
    mutationFn: deleteNode,
    onSuccess: async () => {
      toast.success("节点已删除");
      await queryClient.invalidateQueries({ queryKey: ["nodes"] });
      await queryClient.invalidateQueries({ queryKey: ["status"] });
    },
    onError: (error) => toast.error(error.message),
  });

  const refreshNodeMutation = useMutation({
    mutationFn: refreshNode,
    onSuccess: async () => {
      toast.success("已加入刷新队列");
      await queryClient.invalidateQueries({ queryKey: ["nodes"] });
    },
    onError: (error) => toast.error(error.message),
  });

  const savePromptsMutation = useMutation({
    mutationFn: savePrompts,
    onSuccess: async () => {
      toast.success("提示词规则已保存");
      setPromptSheetOpen(false);
      setEditingPrompt(null);
      await queryClient.invalidateQueries({ queryKey: ["prompts"] });
      await queryClient.invalidateQueries({ queryKey: ["status"] });
    },
    onError: (error) => toast.error(error.message),
  });

  const nodes = nodesQuery.data ?? [];
  const prompts = promptsQuery.data ?? [];
  const healthyNodes = nodes.filter((node) => node.enabled && node.healthy).length;
  const selectedPromptModels = useMemo(() => new Set(prompts.map((rule) => rule.model)), [prompts]);
  const credentialAlerts = nodes.filter((node) => {
    const state = node.credential_status?.state;
    return state === "expired" || state === "expiring" || state === "missing" || state === "invalid";
  });

  useEffect(() => {
    if (statusQuery.error instanceof ApiError && statusQuery.error.status === 401) {
      clearAdminKey();
      toast.error("登录已失效，请重新登录");
    }
  }, [clearAdminKey, statusQuery.error]);

  function submitLogin(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const password = loginPassword.trim();
    if (!password) {
      toast.error("请输入管理员密码");
      return;
    }
    loginMutation.mutate({ password, remember: rememberLogin });
  }

  function logout() {
    clearAdminKey();
    void queryClient.clear();
  }

  function openNewNode() {
    setEditingNode(null);
    setNodeSheetOpen(true);
  }

  function submitNode(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const parsed = nodeSchema.safeParse({
      name: String(form.get("name") ?? ""),
      credential_path: String(form.get("credential_path") ?? ""),
      note: String(form.get("note") ?? ""),
    });
    if (!parsed.success) {
      toast.error("请填写名称和凭据路径");
      return;
    }
    saveNodeMutation.mutate({
      ...editingNode,
      ...parsed.data,
      enabled: form.get("enabled") === "on",
      healthy: editingNode?.healthy ?? true,
    });
  }

  function submitPrompt(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const rule: PromptRule = {
      model: String(form.get("model") ?? "*").trim() || "*",
      enabled: form.get("enabled") === "on",
      mode: String(form.get("mode") ?? "prepend") as PromptRule["mode"],
      content: String(form.get("content") ?? ""),
      note: String(form.get("note") ?? ""),
    };
    const next = prompts.filter((item) => item.model !== editingPrompt?.model && item.model !== rule.model);
    savePromptsMutation.mutate([...next, rule]);
  }

  if (!adminKey) {
    return (
      <LoginView
        isPending={loginMutation.isPending}
        onPasswordChange={setLoginPassword}
        onRememberChange={setRememberLogin}
        onSubmit={submitLogin}
        password={loginPassword}
        remember={rememberLogin}
      />
    );
  }

  return (
    <PageShell activeSection={activeSection}>
      <header className="flex flex-col gap-4 border-b border-border pb-5 md:flex-row md:items-end md:justify-between">
        <div className="flex flex-col gap-2">
          <div className="text-sm font-medium text-muted-foreground">orik / Kiro 反向代理</div>
          <h1 className="text-3xl font-semibold tracking-normal">{currentSection.title}</h1>
          <p className="max-w-2xl text-sm text-muted-foreground">{currentSection.description}</p>
        </div>
        <div className="flex items-center gap-2">
          <Badge variant="outline">
            <ShieldCheckIcon data-icon="inline-start" />
            admin
          </Badge>
          <Button onClick={logout} variant="outline">
            <LogOutIcon data-icon="inline-start" />
            退出
          </Button>
        </div>
      </header>

      {activeSection === "overview" ? (
        <section className="grid gap-4 md:grid-cols-4" id="overview">
          <MetricCard label="健康节点" value={`${healthyNodes}/${nodes.length}`} />
          <MetricCard label="请求数" value={String(statusQuery.data?.requests_total ?? 0)} />
          <MetricCard label="失败数" value={String(statusQuery.data?.failures_total ?? 0)} />
          <MetricCard label="提示词规则" value={String(prompts.length)} />
        </section>
      ) : null}

      {statusQuery.data?.config_warnings?.length ? (
        <Card>
          <CardHeader>
            <CardTitle>配置警告</CardTitle>
            <CardDescription>{statusQuery.data.config_warnings.join(" · ")}</CardDescription>
          </CardHeader>
        </Card>
      ) : null}

      {activeSection === "nodes" ? (
      <Card id="nodes">
        <CardHeader className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
          <div>
            <CardTitle>Kiro 账号池</CardTitle>
            <CardDescription>节点保存在 pools.toml 中，凭据文件保留在挂载的 config 目录。</CardDescription>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button onClick={() => setOauthDialogOpen(true)} variant="outline">
              <ExternalLinkIcon data-icon="inline-start" />
              OAuth
            </Button>
            <Button onClick={() => setCredentialDialogOpen(true)} variant="outline">
              <KeyRoundIcon data-icon="inline-start" />
              导入凭据
            </Button>
            <Button onClick={openNewNode}>
              <PlusIcon data-icon="inline-start" />
              添加节点
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          {credentialAlerts.length ? (
            <div className="mb-4 rounded-md border border-destructive/30 bg-destructive/5 p-3 text-sm">
              <div className="flex items-center gap-2 font-medium text-destructive">
                <AlertTriangleIcon data-icon="inline-start" />
                {credentialAlerts.length} 个节点的凭据需要处理
              </div>
              <p className="mt-1 text-muted-foreground">已过期或缺少 refresh token 的节点需要手动刷新、重新 OAuth，或重新导入凭据。</p>
            </div>
          ) : null}
          <div className="overflow-x-auto rounded-md border border-border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>名称</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>凭据状态</TableHead>
                  <TableHead>凭据</TableHead>
                  <TableHead>用量</TableHead>
                  <TableHead>最近错误</TableHead>
                  <TableHead className="text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {nodes.map((node) => (
                  <TableRow key={node.id}>
                    <TableCell>
                      <div className="font-medium">{node.name}</div>
                      <div className="text-xs text-muted-foreground">{node.id}</div>
                    </TableCell>
                    <TableCell>
                      <NodeStatus node={node} />
                    </TableCell>
                    <TableCell>
                      <CredentialStatusCell status={node.credential_status} />
                    </TableCell>
                    <TableCell className="max-w-xs truncate font-mono text-xs">{node.credential_path}</TableCell>
                    <TableCell>{node.usage_count}</TableCell>
                    <TableCell className="max-w-xs truncate text-muted-foreground">{node.last_error || "无"}</TableCell>
                    <TableCell>
                      <div className="flex justify-end gap-2">
                        <Button onClick={() => refreshNodeMutation.mutate(node.id)} size="sm" variant="outline">
                          <RefreshCwIcon data-icon="inline-start" />
                          刷新
                        </Button>
                        <Button
                          onClick={() => {
                            setEditingNode(node);
                            setNodeSheetOpen(true);
                          }}
                          size="sm"
                          variant="outline"
                        >
                          编辑
                        </Button>
                        <Button onClick={() => deleteNodeMutation.mutate(node.id)} size="sm" variant="destructive">
                          <Trash2Icon data-icon="inline-start" />
                          删除
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
                {nodes.length === 0 ? (
                  <TableRow>
                    <TableCell className="py-8 text-center text-muted-foreground" colSpan={7}>
                      暂未配置 Kiro 节点。
                    </TableCell>
                  </TableRow>
                ) : null}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>
      ) : null}

      {activeSection === "prompts" ? (
      <Card id="prompts">
        <CardHeader className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
          <div>
            <CardTitle>模型系统提示词</CardTitle>
            <CardDescription>规则保存在 prompts.toml 中，精确模型匹配优先于通配规则。</CardDescription>
          </div>
          <Button
            onClick={() => {
              setEditingPrompt(null);
              setPromptSheetOpen(true);
            }}
          >
            <PlusIcon data-icon="inline-start" />
            添加提示词
          </Button>
        </CardHeader>
        <CardContent>
          <div className="grid gap-3">
            {prompts.map((rule) => (
              <button
                className="flex w-full flex-col gap-2 rounded-md border border-border bg-background p-4 text-left transition hover:bg-muted"
                key={rule.model}
                onClick={() => {
                  setEditingPrompt(rule);
                  setPromptSheetOpen(true);
                }}
                type="button"
              >
                <div className="flex flex-wrap items-center gap-2">
                  <span className="font-mono text-sm font-medium">{rule.model}</span>
                  <Badge variant={rule.enabled ? "secondary" : "outline"}>{rule.enabled ? "启用" : "停用"}</Badge>
                  <Badge variant="outline">{promptModeLabels[rule.mode]}</Badge>
                </div>
                <p className="line-clamp-2 text-sm text-muted-foreground">{rule.content || "暂未配置提示词内容。"}</p>
              </button>
            ))}
          </div>
        </CardContent>
      </Card>
      ) : null}

      {activeSection === "access" ? (
      <section id="access">
        <Card>
          <CardHeader>
            <CardTitle>Claude API</CardTitle>
            <CardDescription>Claude 兼容客户端可将管理员密钥作为 Bearer Token 使用。</CardDescription>
          </CardHeader>
          <CardContent>
            <pre className="overflow-x-auto rounded-md bg-muted p-4 text-xs">{`curl http://localhost:13120/v1/messages \\
  -H "Authorization: Bearer <管理员密码>" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"claude-sonnet-4-5","max_tokens":1024,"messages":[{"role":"user","content":"hello"}]}'`}</pre>
            <Button className="mt-3" onClick={() => copyText("http://localhost:13120/v1/messages")} variant="outline">
              <CopyIcon data-icon="inline-start" />
              复制端点
            </Button>
          </CardContent>
        </Card>
      </section>
      ) : null}

      {activeSection === "logs" ? (
      <section id="logs">
        <Card>
          <CardHeader>
            <CardTitle>日志</CardTitle>
            <CardDescription>显示 config/logs/app.log 的尾部内容。</CardDescription>
          </CardHeader>
          <CardContent>
            <pre className="h-72 overflow-auto rounded-md bg-muted p-4 text-xs leading-5 text-muted-foreground">
              {(logsQuery.data?.lines ?? []).join("\n") || "暂无日志。"}
            </pre>
          </CardContent>
        </Card>
      </section>
      ) : null}

      <Sheet onOpenChange={setNodeSheetOpen} open={nodeSheetOpen}>
        <SheetContent>
          <SheetHeader>
            <SheetTitle>{editingNode ? "编辑 Kiro 节点" : "添加 Kiro 节点"}</SheetTitle>
            <SheetDescription>将账号池节点指向已有的 Kiro 凭据 JSON 文件。</SheetDescription>
          </SheetHeader>
          <form className="flex flex-1 flex-col gap-4 overflow-auto px-4" onSubmit={submitNode}>
            <Field label="名称">
              <Input defaultValue={editingNode?.name ?? ""} name="name" placeholder="Kiro 主节点" />
            </Field>
            <Field label="凭据路径">
              <Input defaultValue={editingNode?.credential_path ?? ""} name="credential_path" placeholder="config/credentials/kiro/node.json" />
            </Field>
            <Field label="备注">
              <Textarea defaultValue={editingNode?.note ?? ""} name="note" placeholder="可选的运维备注" />
            </Field>
            <label className="flex items-center gap-2 text-sm">
              <input defaultChecked={editingNode?.enabled ?? true} name="enabled" type="checkbox" />
              启用
            </label>
            <SheetFooter>
              <Button disabled={saveNodeMutation.isPending} type="submit">
                {saveNodeMutation.isPending ? <Loader2Icon data-icon="inline-start" /> : null}
                保存节点
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>

      <Sheet onOpenChange={setPromptSheetOpen} open={promptSheetOpen}>
        <SheetContent>
          <SheetHeader>
            <SheetTitle>{editingPrompt ? "编辑提示词规则" : "添加提示词规则"}</SheetTitle>
            <SheetDescription>使用 * 作为兜底规则；精确模型 ID 优先于通配规则。</SheetDescription>
          </SheetHeader>
          <form className="flex flex-1 flex-col gap-4 overflow-auto px-4" onSubmit={submitPrompt}>
            <Field label="模型">
              <Input
                defaultValue={editingPrompt?.model ?? nextPromptModel(statusQuery.data?.models ?? [], selectedPromptModels)}
                name="model"
                placeholder="claude-sonnet-4-5 或 *"
              />
            </Field>
            <Field label="模式">
              <Select defaultValue={editingPrompt?.mode ?? "prepend"} name="mode">
                {promptModes.map((mode) => (
                  <option key={mode} value={mode}>
                    {promptModeLabels[mode]}
                  </option>
                ))}
              </Select>
            </Field>
            <Field label="系统提示词">
              <Textarea defaultValue={editingPrompt?.content ?? ""} name="content" placeholder="这个模型使用的系统提示词" />
            </Field>
            <Field label="备注">
              <Input defaultValue={editingPrompt?.note ?? ""} name="note" placeholder="可选备注" />
            </Field>
            <label className="flex items-center gap-2 text-sm">
              <input defaultChecked={editingPrompt?.enabled ?? true} name="enabled" type="checkbox" />
              启用
            </label>
            <SheetFooter>
              <Button disabled={savePromptsMutation.isPending} type="submit">
                保存提示词
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>

      <CredentialImportDialog onOpenChange={setCredentialDialogOpen} open={credentialDialogOpen} />
      <OAuthDialog onOpenChange={setOauthDialogOpen} open={oauthDialogOpen} />
    </PageShell>
  );
}

function LoginView({
  isPending,
  onPasswordChange,
  onRememberChange,
  onSubmit,
  password,
  remember,
}: {
  isPending: boolean;
  onPasswordChange: (value: string) => void;
  onRememberChange: (value: boolean) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  password: string;
  remember: boolean;
}) {
  return (
    <main className="flex min-h-screen items-center justify-center px-4">
      <Card className="w-full max-w-md">
        <CardHeader>
          <div className="mb-2 flex size-10 items-center justify-center rounded-md bg-primary text-primary-foreground">
            <KeyRoundIcon />
          </div>
          <CardTitle>登录 orik</CardTitle>
          <CardDescription>使用 config.toml 中的 admin_api_key 作为 admin 密码。</CardDescription>
        </CardHeader>
        <CardContent>
          <form className="flex flex-col gap-4" onSubmit={onSubmit}>
            <Field label="管理员账号">
              <Input disabled value="admin" />
            </Field>
            <Field label="管理员密码">
              <Input
                autoComplete="current-password"
                autoFocus
                onChange={(event) => onPasswordChange(event.target.value)}
                placeholder="输入 admin_api_key"
                type="password"
                value={password}
              />
            </Field>
            <label className="flex items-start gap-2 text-sm text-muted-foreground">
              <input checked={remember} onChange={(event) => onRememberChange(event.target.checked)} type="checkbox" />
              <span>保持登录。默认只保存到当前浏览器会话，勾选后才写入本机 localStorage。</span>
            </label>
            <Button disabled={isPending} type="submit">
              {isPending ? <Loader2Icon data-icon="inline-start" /> : <KeyRoundIcon data-icon="inline-start" />}
              登录
            </Button>
          </form>
        </CardContent>
      </Card>
    </main>
  );
}

function useActiveSection() {
  const [activeSection, setActiveSection] = useState<NavSection>(() => readActiveSection());

  useEffect(() => {
    function syncHash() {
      setActiveSection(readActiveSection());
      window.scrollTo({ top: 0 });
    }

    syncHash();
    window.addEventListener("hashchange", syncHash);
    return () => window.removeEventListener("hashchange", syncHash);
  }, []);

  return activeSection;
}

function readActiveSection(): NavSection {
  if (typeof window === "undefined") {
    return "overview";
  }
  const hash = window.location.hash.replace(/^#/, "");
  return sectionIds.includes(hash as NavSection) ? (hash as NavSection) : "overview";
}

function MetricCard({ label, value }: { label: string; value: string }) {
  return (
    <Card>
      <CardHeader>
        <CardDescription>{label}</CardDescription>
        <CardTitle className="text-2xl">{value}</CardTitle>
      </CardHeader>
    </Card>
  );
}

function NodeStatus({ node }: { node: KiroNode }) {
  if (!node.enabled) {
    return <Badge variant="outline">停用</Badge>;
  }
  if (node.healthy) {
    return (
      <Badge variant="secondary">
        <CheckCircle2Icon data-icon="inline-start" />
        健康
      </Badge>
    );
  }
  return (
    <Badge variant="destructive">
      <XCircleIcon data-icon="inline-start" />
      异常
    </Badge>
  );
}

function CredentialStatusCell({ status }: { status?: CredentialStatus }) {
  if (!status) {
    return <Badge variant="outline">未知</Badge>;
  }
  const label = credentialStateLabel(status.state);
  const variant = status.state === "active" ? "secondary" : status.state === "unknown" ? "outline" : "destructive";
  const Icon = status.state === "active" ? CheckCircle2Icon : status.state === "expiring" ? ClockIcon : AlertTriangleIcon;
  return (
    <div className="flex max-w-xs flex-col gap-1">
      <Badge variant={variant}>
        <Icon data-icon="inline-start" />
        {label}
      </Badge>
      <div className="text-xs text-muted-foreground">
        {status.expires_at ? `过期时间：${formatDateTime(status.expires_at)}` : status.message}
      </div>
      {!status.refreshable && status.state !== "active" ? <div className="text-xs text-destructive">缺少 refresh token，需要重新授权或导入。</div> : null}
    </div>
  );
}

function credentialStateLabel(state: CredentialStatus["state"]) {
  switch (state) {
    case "active":
      return "有效";
    case "expiring":
      return "即将过期";
    case "expired":
      return "已过期";
    case "missing":
      return "文件缺失";
    case "invalid":
      return "无效";
    case "unknown":
      return "未知";
  }
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="flex flex-col gap-2 text-sm font-medium">
      {label}
      {children}
    </label>
  );
}

function CredentialImportDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const queryClient = useQueryClient();
  const [name, setName] = useState("");
  const [text, setText] = useState("");
  const mutation = useMutation({
    mutationFn: importCredentials,
    onSuccess: async () => {
      toast.success("凭据已导入");
      setText("");
      setName("");
      onOpenChange(false);
      await queryClient.invalidateQueries({ queryKey: ["nodes"] });
      await queryClient.invalidateQueries({ queryKey: ["status"] });
    },
    onError: (error) => toast.error(error.message),
  });

  function submit() {
    try {
      mutation.mutate({ name: name || undefined, credentials: JSON.parse(text) as unknown });
    } catch {
      toast.error("凭据必须是有效的 JSON");
    }
  }

  return (
    <Dialog onOpenChange={onOpenChange} open={open}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>导入 Kiro 凭据</DialogTitle>
          <DialogDescription>粘贴一个凭据对象，或粘贴凭据对象数组。</DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-4">
          <Field label="节点名称">
            <Input onChange={(event) => setName(event.target.value)} placeholder="Kiro 导入节点" value={name} />
          </Field>
          <Field label="凭据 JSON">
            <Textarea onChange={(event) => setText(event.target.value)} placeholder='{"accessToken":"...","refreshToken":"..."}' value={text} />
          </Field>
        </div>
        <DialogFooter>
          <Button disabled={mutation.isPending} onClick={submit}>
            导入
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function OAuthDialog({ open, onOpenChange }: { open: boolean; onOpenChange: (open: boolean) => void }) {
  const [method, setMethod] = useState("google");
  const [nodeName, setNodeName] = useState("");
  const mutation = useMutation({
    mutationFn: startOAuth,
    onSuccess: (result) => {
      toast.success("OAuth 流程已启动");
      window.open(result.auth_url, "_blank", "noopener,noreferrer");
    },
    onError: (error) => toast.error(error.message),
  });

  return (
    <Dialog onOpenChange={onOpenChange} open={open}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>启动 Kiro OAuth</DialogTitle>
          <DialogDescription>社交登录会打开浏览器回调，Builder ID 会返回设备授权地址。</DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-4">
          <Field label="方式">
            <Select onChange={(event) => setMethod(event.target.value)} value={method}>
              <option value="google">Google</option>
              <option value="github">GitHub</option>
              <option value="builder-id">Builder ID</option>
            </Select>
          </Field>
          <Field label="节点名称">
            <Input onChange={(event) => setNodeName(event.target.value)} placeholder="Kiro OAuth" value={nodeName} />
          </Field>
        </div>
        <DialogFooter>
          <Button disabled={mutation.isPending} onClick={() => mutation.mutate({ method, node_name: nodeName || undefined })}>
            <ExternalLinkIcon data-icon="inline-start" />
            开始
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function nextPromptModel(models: string[], selected: Set<string>) {
  return models.find((model) => !selected.has(model)) ?? "*";
}

function formatDateTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString("zh-CN", { hour12: false });
}

async function copyText(value: string) {
  await navigator.clipboard.writeText(value);
  toast.success("已复制");
}
