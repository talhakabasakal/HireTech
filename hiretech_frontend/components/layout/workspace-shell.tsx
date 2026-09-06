"use client";

import { useEffect, useState, type ReactNode } from "react";
import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { BarChart3, Bot, Building2, ChevronLeft, ChevronRight, ChevronsUpDown, ClipboardCheck, Laptop, LayoutDashboard, LogOut, Menu, MessageSquareText, Plus, Settings2, ShieldCheck, UserRound, UsersRound, X } from "lucide-react";
import { AdminTabs } from "@/components/layout/admin-tabs";
import { Brand } from "@/components/layout/brand";
import { LoadingState } from "@/components/states/async-state";
import { cn } from "@/components/ui/utils";
import { dataMode } from "@/core/config/runtime";
import { logoutCurrentSession } from "@/core/infrastructure/api/auth-repositories";
import { apiRequest } from "@/core/infrastructure/api/http-client";
import { decodeSessionToken, readSessionTokens } from "@/core/infrastructure/api/session-store";
import { homeRouteForRole, roleFromClaims as resolveRoleFromClaims } from "@/core/permissions/permissions";
import type { UserRole } from "@/core/domain/identity";
import styles from "@/components/layout/workspace-shell.module.css";

type Area = "candidate" | "recruiter" | "admin";
type ShellVariant = "workspace" | "candidate-portal";
type NavigationIcon = typeof LayoutDashboard;

type WorkspaceUser = { id: string; email: string; first_name: string; last_name: string };
interface WorkspaceIdentity { displayName: string; email: string; role: UserRole; organizationName: string }
interface NavigationItem { href: string; label: string; icon: NavigationIcon }
interface NavigationGroup { label: string; items: NavigationItem[] }

const navigation: Record<Area, NavigationGroup[]> = {
  candidate: [
    { label: "Workspace", items: [{ href: "/candidate", label: "Overview", icon: LayoutDashboard }, { href: "/candidate/invitation", label: "Invitation", icon: MessageSquareText }, { href: "/candidate/preparation", label: "Preparation", icon: ClipboardCheck }] },
    { label: "Account", items: [{ href: "/settings/devices", label: "Registered devices", icon: Laptop }] },
  ],
  recruiter: [
    { label: "İşe alım", items: [{ href: "/recruiter", label: "Genel bakış", icon: LayoutDashboard }, { href: "/recruiter/interviews", label: "Mülakatlar", icon: ClipboardCheck }, { href: "/recruiter/interviews/new", label: "Yeni mülakat", icon: Plus }] },
    { label: "İnceleme", items: [{ href: "/recruiter/candidates", label: "Adaylar & raporlar", icon: UsersRound }] },
    { label: "Sistem", items: [{ href: "/admin/models", label: "Yönetim paneli", icon: Settings2 }, { href: "/settings/devices", label: "Kayıtlı cihazlar", icon: Laptop }] },
  ],
  admin: [
    { label: "AI control", items: [{ href: "/admin/models", label: "Model registry", icon: Bot }, { href: "/admin/prompts/interviewer", label: "Prompt management", icon: MessageSquareText }, { href: "/admin/rubrics", label: "Evaluation rubrics", icon: ClipboardCheck }, { href: "/admin/routing", label: "Model routing", icon: Settings2 }] },
    { label: "Governance", items: [{ href: "/admin/audit", label: "Audit log", icon: BarChart3 }] },
  ],
};

function roleFromClaims(token: string | undefined): UserRole {
  const claims = decodeSessionToken(token);
  return resolveRoleFromClaims(claims?.roles, claims?.permissions, claims?.tokenClass);
}

function areaAllowsRole(area: Area, role: UserRole): boolean {
  return area === role || (area === "admin" && role === "admin");
}

function returnTo(pathname: string): string {
  if (typeof window === "undefined") return pathname;
  return `${pathname}${window.location.search}`;
}

export function WorkspaceShell({ area, children, variant = "workspace" }: { area: Area; children: ReactNode; variant?: ShellVariant }) {
  const pathname = usePathname();
  const router = useRouter();
  const [collapsed, setCollapsed] = useState(false);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [authState, setAuthState] = useState<"loading" | "ready">(dataMode === "mock" ? "ready" : "loading");
  const [identity, setIdentity] = useState<WorkspaceIdentity | null>(null);
  const [loggingOut, setLoggingOut] = useState(false);

  useEffect(() => {
    if (dataMode === "mock") return;
    let cancelled = false;
    const bootstrap = async () => {
      try {
        const tokens = await readSessionTokens();
        if (!tokens?.accessToken) {
          router.replace(`/login?returnTo=${encodeURIComponent(returnTo(pathname))}`);
          return;
        }
        const user = await apiRequest<WorkspaceUser>("/api/v1/me", {}, true);
        const role = roleFromClaims(tokens.accessToken);
        if (!areaAllowsRole(area, role)) {
          router.replace(homeRouteForRole(role));
          return;
        }
        if (!cancelled) {
          const claims = decodeSessionToken(tokens.accessToken);
          setIdentity({ displayName: `${user.first_name} ${user.last_name}`.trim() || user.email, email: user.email, role, organizationName: claims?.organizationId ? "Current organization" : "Invitation workspace" });
          setAuthState("ready");
        }
      } catch {
        if (!cancelled) router.replace(`/login?returnTo=${encodeURIComponent(returnTo(pathname))}`);
      }
    };
    void bootstrap();
    return () => { cancelled = true; };
  }, [area, pathname, router]);

  useEffect(() => {
    if (!drawerOpen) return;
    const handleKeyDown = (event: KeyboardEvent) => { if (event.key === "Escape") setDrawerOpen(false); };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [drawerOpen]);

  const signOut = async () => {
    setLoggingOut(true);
    try {
      if (dataMode === "api") await logoutCurrentSession();
    } catch {
      // Local credentials are cleared by logoutCurrentSession even when the API is unavailable.
    } finally {
      setLoggingOut(false);
      router.replace("/login");
    }
  };

  if (authState === "loading") return <main className="interview-loading"><LoadingState label="Verifying your session" /></main>;

  const isActive = (href: string) => {
    if (area === "recruiter" && href === "/recruiter/candidates") {
      return pathname.startsWith("/recruiter/candidates") || pathname.startsWith("/recruiter/reports") || pathname.startsWith("/recruiter/reviews");
    }
    return href === `/${area}` ? pathname === href : pathname.startsWith(href);
  };
  const renderNavigation = (mobile = false) => <nav className={mobile ? styles.drawerNavigation : styles.navigation} aria-label={`${area}${mobile ? " mobile" : ""} navigation`}>
    {navigation[area].map((group) => <section className={styles.group} key={group.label} aria-label={group.label}><p className={styles.groupLabel}>{group.label}</p>{group.items.map(({ href, label, icon: Icon }) => {
      const active = isActive(href);
      return <Link key={href} href={href} className={cn(styles.navItem, active && styles.active)} aria-current={active ? "page" : undefined} data-tooltip={label} onClick={() => setDrawerOpen(false)}><Icon aria-hidden="true" /><span className={styles.navItemLabel}>{label}</span></Link>;
    })}</section>)}
  </nav>;

  const currentIdentity = identity ?? { displayName: "Alex Morgan", email: "", role: "candidate" as UserRole, organizationName: "Current organization" };
  if (variant === "candidate-portal") {
    const portalNavigation = navigation.candidate[0]?.items ?? [];
    const isMockMode = dataMode === "mock";
    return <div className="candidate-portal-shell">
      <header className="candidate-portal-header">
        <div className="candidate-portal-header-inner">
          <Brand href="/candidate" />
          <nav className="candidate-portal-nav" aria-label="Candidate portal navigation">
            {portalNavigation.map(({ href, label, icon: Icon }) => {
              const active = isActive(href);
              return <Link key={href} href={href} className={cn("candidate-portal-nav-link", active && "active")} aria-current={active ? "page" : undefined}><Icon aria-hidden="true" /><span>{label}</span></Link>;
            })}
          </nav>
          <div className="candidate-portal-account"><span className={cn("candidate-portal-secure", isMockMode && "candidate-portal-demo")}><ShieldCheck aria-hidden="true" />{isMockMode ? "Demo data only" : "Secure session"}</span><span className="candidate-portal-user">{isMockMode ? "Demo account" : currentIdentity.displayName}</span><button type="button" className="candidate-portal-signout" onClick={() => void signOut()} disabled={loggingOut}>Sign out</button></div>
        </div>
      </header>
      <div className="candidate-portal-content">{isMockMode && <div className="candidate-portal-mode-banner" role="status">Demo mode — isolated UI data only. Nothing is sent to or read from a backend.</div>}{children}</div>
      <footer className="candidate-portal-footer"><div><strong>HireTech candidate portal</strong><span>{isMockMode ? "Isolated UI demo. No backend or tenant data is used." : "Your interview workspace is private and connected to the hiring team’s secure system."}</span></div><Link href="/settings/devices">Device settings</Link></footer>
    </div>;
  }
  const organization = <div className={styles.organization}><span className={styles.organizationIcon}><Building2 aria-hidden="true" /></span><div className={styles.organizationCopy}><strong>{currentIdentity.organizationName}</strong><span>{area.charAt(0).toUpperCase() + area.slice(1)} workspace</span></div><ChevronsUpDown aria-hidden="true" /></div>;
  const sessionLabel = dataMode === "mock" ? "Local mock data only" : "Connected to protected API";

  return <div className={cn("workspace-shell", collapsed && "sidebar-collapsed")}>
    <aside className={cn(styles.sidebar, collapsed && styles.collapsed)}>
      <div className={styles.brandArea}><Brand /><button type="button" className={styles.collapseButton} onClick={() => setCollapsed((value) => !value)} aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"} aria-expanded={!collapsed}>{collapsed ? <ChevronRight /> : <ChevronLeft />}</button></div>
      {organization}
      {renderNavigation()}
      <footer className={styles.footer}><div className={styles.sessionStatus}><ShieldCheck aria-hidden="true" /><div><strong>Verified session</strong><span>{sessionLabel}</span></div></div><div className={styles.profile}><span className={styles.avatar}>{currentIdentity.displayName.split(" ").map((part) => part[0]).join("").slice(0, 2).toUpperCase()}</span><div className={styles.profileCopy}><strong>{currentIdentity.displayName}</strong><span>{dataMode === "mock" ? "Demo account" : currentIdentity.email}</span></div><button type="button" className={styles.signOut} aria-label="Sign out" title="Sign out" onClick={() => void signOut()} disabled={loggingOut}><LogOut aria-hidden="true" /></button></div></footer>
    </aside>
    {drawerOpen && <button className="drawer-overlay" type="button" onClick={() => setDrawerOpen(false)} aria-label="Close navigation" />}
    {drawerOpen && <aside className="mobile-drawer open" aria-label="Mobile navigation"><div className="mobile-drawer-header"><Brand /><button type="button" onClick={() => setDrawerOpen(false)} aria-label="Close navigation"><X /></button></div>{organization}{renderNavigation(true)}</aside>}
    <div className="workspace-main"><header className="workspace-header"><button className="mobile-menu-button" type="button" onClick={() => setDrawerOpen(true)} aria-label="Open navigation" aria-expanded={drawerOpen}><Menu /></button><div className="workspace-context"><span>{area}</span><strong>{currentIdentity.organizationName}</strong></div><div className="header-account"><span>{dataMode === "mock" ? "Demo environment" : "Protected workspace"}</span><button type="button" aria-label="Open account menu"><UserRound /></button></div></header>{area === "admin" && <div className="admin-tabs-wrap"><AdminTabs /></div>}{children}</div>
  </div>;
}
