import type { ReactNode } from "react";
import { ActivityIcon, FileTextIcon, KeyRoundIcon, ScrollTextIcon, ServerIcon } from "lucide-react";

import { cn } from "@/lib/utils";

export type NavSection = "overview" | "nodes" | "prompts" | "access" | "logs";

const navItems = [
  { href: "#overview", id: "overview", label: "概览", icon: ActivityIcon },
  { href: "#nodes", id: "nodes", label: "Kiro 账号池", icon: ServerIcon },
  { href: "#prompts", id: "prompts", label: "提示词", icon: FileTextIcon },
  { href: "#access", id: "access", label: "访问配置", icon: KeyRoundIcon },
  { href: "#logs", id: "logs", label: "日志", icon: ScrollTextIcon },
] satisfies Array<{ href: `#${NavSection}`; id: NavSection; label: string; icon: typeof ActivityIcon }>;

export function PageShell({ activeSection, children }: { activeSection: NavSection; children: ReactNode }) {
  return (
    <div className="min-h-screen">
      <aside className="fixed inset-y-0 left-0 hidden w-64 border-r border-border bg-card/80 px-4 py-5 backdrop-blur lg:block">
        <div className="flex flex-col gap-1">
          <div className="text-lg font-semibold">orik</div>
          <div className="text-sm text-muted-foreground">Claude 兼容本地网关</div>
        </div>
        <nav aria-label="主导航" className="mt-8 flex flex-col gap-1">
          {navItems.map((item) => {
            const Icon = item.icon;
            const isActive = activeSection === item.id;
            return (
              <a
                aria-current={isActive ? "page" : undefined}
                className={cn(
                  "flex items-center gap-2 rounded-md px-3 py-2 text-sm text-muted-foreground transition hover:bg-muted hover:text-foreground",
                  isActive && "bg-muted font-medium text-foreground",
                )}
                href={item.href}
                key={item.href}
              >
                <Icon />
                {item.label}
              </a>
            );
          })}
        </nav>
      </aside>
      <main className="lg:pl-64">
        <div className="mx-auto flex w-full max-w-7xl flex-col gap-6 px-4 py-5 sm:px-6 lg:px-8">{children}</div>
      </main>
    </div>
  );
}
