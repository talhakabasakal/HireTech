"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { cn } from "@/components/ui/utils";

const tabs = [
  { href: "/admin/models", label: "Models" },
  { href: "/admin/prompts/interviewer", label: "Prompts", match: "/admin/prompts" },
  { href: "/admin/rubrics", label: "Rubrics" },
  { href: "/admin/routing", label: "Routing" },
  { href: "/admin/versions", label: "Versions" },
  { href: "/admin/audit", label: "Audit" },
] as const;

export function AdminTabs() {
  const pathname = usePathname();
  return <nav className="admin-tabs" aria-label="LLM configuration sections">
    {tabs.map((tab) => <Link key={tab.href} href={tab.href} className={cn(pathname.startsWith("match" in tab ? tab.match : tab.href) && "active")}>{tab.label}</Link>)}
  </nav>;
}
