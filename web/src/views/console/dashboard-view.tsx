import { FormEvent, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  CheckCircle2Icon,
  CopyIcon,
  ExternalLinkIcon,
  FileTextIcon,
  KeyRoundIcon,
  Loader2Icon,
  PlusIcon,
  RefreshCwIcon,
  ServerIcon,
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
import type { KiroNode } from "@/features/pools/types";
import { listPrompts, savePrompts } from "@/features/prompts/api";
import type { PromptRule } from "@/features/prompts/types";
import { getLogTail, getStatus } from "@/features/status/api";
import { PageShell } from "@/shared/layout/page-shell";
import { useSessionStore } from "@/shared/api/session-store";

const nodeSchema = z.object({
  name: z.string().min(1),
  credential_path: z.string().min(1),
  note: z.string().optional(),
});

const promptModes: PromptRule["mode"][] = ["prepend", "append", "override", "off"];

export function DashboardView() {
  const queryClient = useQueryClient();
  const { adminKey, setAdminKey } = useSessionStore();
  const [draftKey, setDraftKey] = useState(adminKey);
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

  const saveNodeMutation = useMutation({
    mutationFn: saveNode,
    onSuccess: async () => {
      toast.success("Kiro node saved");
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
      toast.success("Node deleted");
      await queryClient.invalidateQueries({ queryKey: ["nodes"] });
      await queryClient.invalidateQueries({ queryKey: ["status"] });
    },
    onError: (error) => toast.error(error.message),
  });

  const refreshNodeMutation = useMutation({
    mutationFn: refreshNode,
    onSuccess: async () => {
      toast.success("Refresh queued");
      await queryClient.invalidateQueries({ queryKey: ["nodes"] });
    },
    onError: (error) => toast.error(error.message),
  });

  const savePromptsMutation = useMutation({
    mutationFn: savePrompts,
    onSuccess: async () => {
      toast.success("Prompt rules saved");
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

  function saveKey(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setAdminKey(draftKey.trim());
    void queryClient.invalidateQueries();
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
      toast.error("Name and credential path are required");
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

  return (
    <PageShell>
      <header className="flex flex-col gap-4 border-b border-border pb-5 md:flex-row md:items-end md:justify-between">
        <div className="flex flex-col gap-2">
          <div className="text-sm font-medium text-muted-foreground">Kiro reverse proxy</div>
          <h1 className="text-3xl font-semibold tracking-normal">AIClient Kiro Console</h1>
          <p className="max-w-2xl text-sm text-muted-foreground">
            Manage Kiro account pools, model system prompts, OAuth onboarding, logs, and Claude-compatible API status.
          </p>
        </div>
        <form className="flex w-full gap-2 md:w-auto" onSubmit={saveKey}>
          <Input
            aria-label="Admin API key"
            className="md:w-72"
            onChange={(event) => setDraftKey(event.target.value)}
            placeholder="Admin API key"
            type="password"
            value={draftKey}
          />
          <Button type="submit">
            <KeyRoundIcon data-icon="inline-start" />
            Apply
          </Button>
        </form>
      </header>

      <section className="grid gap-4 md:grid-cols-4" id="overview">
        <MetricCard label="Healthy nodes" value={`${healthyNodes}/${nodes.length}`} />
        <MetricCard label="Requests" value={String(statusQuery.data?.requests_total ?? 0)} />
        <MetricCard label="Failures" value={String(statusQuery.data?.failures_total ?? 0)} />
        <MetricCard label="Prompt rules" value={String(prompts.length)} />
      </section>

      {statusQuery.data?.config_warnings?.length ? (
        <Card>
          <CardHeader>
            <CardTitle>Configuration warnings</CardTitle>
            <CardDescription>{statusQuery.data.config_warnings.join(" · ")}</CardDescription>
          </CardHeader>
        </Card>
      ) : null}

      <Card id="nodes">
        <CardHeader className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
          <div>
            <CardTitle>Kiro account pool</CardTitle>
            <CardDescription>Nodes are persisted in pools.toml. Credential files stay in the mounted config directory.</CardDescription>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button onClick={() => setOauthDialogOpen(true)} variant="outline">
              <ExternalLinkIcon data-icon="inline-start" />
              OAuth
            </Button>
            <Button onClick={() => setCredentialDialogOpen(true)} variant="outline">
              <KeyRoundIcon data-icon="inline-start" />
              Import
            </Button>
            <Button onClick={openNewNode}>
              <PlusIcon data-icon="inline-start" />
              Add node
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto rounded-md border border-border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Credential</TableHead>
                  <TableHead>Usage</TableHead>
                  <TableHead>Last error</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
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
                    <TableCell className="max-w-xs truncate font-mono text-xs">{node.credential_path}</TableCell>
                    <TableCell>{node.usage_count}</TableCell>
                    <TableCell className="max-w-xs truncate text-muted-foreground">{node.last_error || "None"}</TableCell>
                    <TableCell>
                      <div className="flex justify-end gap-2">
                        <Button onClick={() => refreshNodeMutation.mutate(node.id)} size="sm" variant="outline">
                          <RefreshCwIcon data-icon="inline-start" />
                          Refresh
                        </Button>
                        <Button
                          onClick={() => {
                            setEditingNode(node);
                            setNodeSheetOpen(true);
                          }}
                          size="sm"
                          variant="outline"
                        >
                          Edit
                        </Button>
                        <Button onClick={() => deleteNodeMutation.mutate(node.id)} size="sm" variant="destructive">
                          <Trash2Icon data-icon="inline-start" />
                          Delete
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
                {nodes.length === 0 ? (
                  <TableRow>
                    <TableCell className="py-8 text-center text-muted-foreground" colSpan={6}>
                      No Kiro nodes configured.
                    </TableCell>
                  </TableRow>
                ) : null}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>

      <Card id="prompts">
        <CardHeader className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
          <div>
            <CardTitle>Model system prompts</CardTitle>
            <CardDescription>Rules are persisted in prompts.toml. Exact model matches override the wildcard rule.</CardDescription>
          </div>
          <Button
            onClick={() => {
              setEditingPrompt(null);
              setPromptSheetOpen(true);
            }}
          >
            <PlusIcon data-icon="inline-start" />
            Add prompt
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
                  <Badge variant={rule.enabled ? "secondary" : "outline"}>{rule.enabled ? "enabled" : "disabled"}</Badge>
                  <Badge variant="outline">{rule.mode}</Badge>
                </div>
                <p className="line-clamp-2 text-sm text-muted-foreground">{rule.content || "No prompt content configured."}</p>
              </button>
            ))}
          </div>
        </CardContent>
      </Card>

      <section className="grid gap-4 lg:grid-cols-[1fr_1.3fr]" id="access">
        <Card>
          <CardHeader>
            <CardTitle>Claude API</CardTitle>
            <CardDescription>Use the admin API key as a bearer token for Claude-compatible clients.</CardDescription>
          </CardHeader>
          <CardContent>
            <pre className="overflow-x-auto rounded-md bg-muted p-4 text-xs">{`curl http://localhost:13120/v1/messages \\
  -H "Authorization: Bearer ${adminKey || "change-me"}" \\
  -H "Content-Type: application/json" \\
  -d '{"model":"claude-sonnet-4-5","max_tokens":1024,"messages":[{"role":"user","content":"hello"}]}'`}</pre>
            <Button className="mt-3" onClick={() => copyText("http://localhost:13120/v1/messages")} variant="outline">
              <CopyIcon data-icon="inline-start" />
              Copy endpoint
            </Button>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Logs</CardTitle>
            <CardDescription>Tail of config/logs/app.log.</CardDescription>
          </CardHeader>
          <CardContent>
            <pre className="h-72 overflow-auto rounded-md bg-muted p-4 text-xs leading-5 text-muted-foreground">
              {(logsQuery.data?.lines ?? []).join("\n") || "No logs yet."}
            </pre>
          </CardContent>
        </Card>
      </section>

      <Sheet onOpenChange={setNodeSheetOpen} open={nodeSheetOpen}>
        <SheetContent>
          <SheetHeader>
            <SheetTitle>{editingNode ? "Edit Kiro node" : "Add Kiro node"}</SheetTitle>
            <SheetDescription>Point a pool node at an existing Kiro credential JSON file.</SheetDescription>
          </SheetHeader>
          <form className="flex flex-1 flex-col gap-4 overflow-auto px-4" onSubmit={submitNode}>
            <Field label="Name">
              <Input defaultValue={editingNode?.name ?? ""} name="name" placeholder="Kiro primary" />
            </Field>
            <Field label="Credential path">
              <Input defaultValue={editingNode?.credential_path ?? ""} name="credential_path" placeholder="config/credentials/kiro/node.json" />
            </Field>
            <Field label="Note">
              <Textarea defaultValue={editingNode?.note ?? ""} name="note" placeholder="Optional operator note" />
            </Field>
            <label className="flex items-center gap-2 text-sm">
              <input defaultChecked={editingNode?.enabled ?? true} name="enabled" type="checkbox" />
              Enabled
            </label>
            <SheetFooter>
              <Button disabled={saveNodeMutation.isPending} type="submit">
                {saveNodeMutation.isPending ? <Loader2Icon data-icon="inline-start" /> : null}
                Save node
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>

      <Sheet onOpenChange={setPromptSheetOpen} open={promptSheetOpen}>
        <SheetContent>
          <SheetHeader>
            <SheetTitle>{editingPrompt ? "Edit prompt rule" : "Add prompt rule"}</SheetTitle>
            <SheetDescription>Use * as the fallback rule. Exact model IDs win over the wildcard.</SheetDescription>
          </SheetHeader>
          <form className="flex flex-1 flex-col gap-4 overflow-auto px-4" onSubmit={submitPrompt}>
            <Field label="Model">
              <Input
                defaultValue={editingPrompt?.model ?? nextPromptModel(statusQuery.data?.models ?? [], selectedPromptModels)}
                name="model"
                placeholder="claude-sonnet-4-5 or *"
              />
            </Field>
            <Field label="Mode">
              <Select defaultValue={editingPrompt?.mode ?? "prepend"} name="mode">
                {promptModes.map((mode) => (
                  <option key={mode} value={mode}>
                    {mode}
                  </option>
                ))}
              </Select>
            </Field>
            <Field label="System prompt">
              <Textarea defaultValue={editingPrompt?.content ?? ""} name="content" placeholder="System prompt for this model" />
            </Field>
            <Field label="Note">
              <Input defaultValue={editingPrompt?.note ?? ""} name="note" placeholder="Optional note" />
            </Field>
            <label className="flex items-center gap-2 text-sm">
              <input defaultChecked={editingPrompt?.enabled ?? true} name="enabled" type="checkbox" />
              Enabled
            </label>
            <SheetFooter>
              <Button disabled={savePromptsMutation.isPending} type="submit">
                Save prompt
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
    return <Badge variant="outline">disabled</Badge>;
  }
  if (node.healthy) {
    return (
      <Badge variant="secondary">
        <CheckCircle2Icon data-icon="inline-start" />
        healthy
      </Badge>
    );
  }
  return (
    <Badge variant="destructive">
      <XCircleIcon data-icon="inline-start" />
      unhealthy
    </Badge>
  );
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
      toast.success("Credentials imported");
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
      toast.error("Credentials must be valid JSON");
    }
  }

  return (
    <Dialog onOpenChange={onOpenChange} open={open}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Import Kiro credentials</DialogTitle>
          <DialogDescription>Paste one credential object or an array of credential objects.</DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-4">
          <Field label="Node name">
            <Input onChange={(event) => setName(event.target.value)} placeholder="Kiro imported" value={name} />
          </Field>
          <Field label="Credentials JSON">
            <Textarea onChange={(event) => setText(event.target.value)} placeholder='{"accessToken":"...","refreshToken":"..."}' value={text} />
          </Field>
        </div>
        <DialogFooter>
          <Button disabled={mutation.isPending} onClick={submit}>
            Import
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
      toast.success("OAuth flow started");
      window.open(result.auth_url, "_blank", "noopener,noreferrer");
    },
    onError: (error) => toast.error(error.message),
  });

  return (
    <Dialog onOpenChange={onOpenChange} open={open}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Start Kiro OAuth</DialogTitle>
          <DialogDescription>Social auth opens a browser callback. Builder ID returns a device authorization URL.</DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-4">
          <Field label="Method">
            <Select onChange={(event) => setMethod(event.target.value)} value={method}>
              <option value="google">Google</option>
              <option value="github">GitHub</option>
              <option value="builder-id">Builder ID</option>
            </Select>
          </Field>
          <Field label="Node name">
            <Input onChange={(event) => setNodeName(event.target.value)} placeholder="Kiro OAuth" value={nodeName} />
          </Field>
        </div>
        <DialogFooter>
          <Button disabled={mutation.isPending} onClick={() => mutation.mutate({ method, node_name: nodeName || undefined })}>
            <ExternalLinkIcon data-icon="inline-start" />
            Start
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function nextPromptModel(models: string[], selected: Set<string>) {
  return models.find((model) => !selected.has(model)) ?? "*";
}

async function copyText(value: string) {
  await navigator.clipboard.writeText(value);
  toast.success("Copied");
}
