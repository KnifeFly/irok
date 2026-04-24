import type { ReactNode } from "react";
import { ActivityIcon, FileTextIcon, KeyRoundIcon, ServerIcon } from "lucide-react";

import { cn } from "@/lib/utils";

const navItems = [
  { href: "#overview", label: "Overview", icon: ActivityIcon },
  { href: "#nodes", label: "Kiro Pool", icon: ServerIcon },
  { href: "#prompts", label: "Prompts", icon: FileTextIcon },
  { href: "#access", label: "Access", icon: KeyRoundIcon },
];

export function PageShell({ children }: { children: ReactNode }) {
  return (
    <div className="min-h-screen">
      <aside className="fixed inset-y-0 left-0 hidden w-64 border-r border-border bg-card/80 px-4 py-5 backdrop-blur lg:block">
        <div className="flex flex-col gap-1">
          <div className="text-lg font-semibold">AIClient Kiro</div>
          <div className="text-sm text-muted-foreground">Claude-compatible local gateway</div>
        </div>
        <nav className="mt-8 flex flex-col gap-1">
          {navItems.map((item) => {
            const Icon = item.icon;
            return (
              <a
                className={cn(
                  "flex items-center gap-2 rounded-md px-3 py-2 text-sm text-muted-foreground transition hover:bg-muted hover:text-foreground",
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
