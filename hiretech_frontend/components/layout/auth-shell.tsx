import type { ReactNode } from "react";
import { ShieldCheck, Sparkles, UsersRound } from "lucide-react";
import { Brand } from "@/components/layout/brand";
import { dataMode } from "@/core/config/runtime";

export function AuthShell({ children, eyebrow, title, description }: { children: ReactNode; eyebrow: string; title: string; description: string }) {
  return <main className="auth-shell">
    <section className="auth-story" aria-label="HireTech overview">
      <Brand />
      <div className="auth-story-content"><p className="eyebrow">Structured technical hiring</p><h1>Better signal.<br />Fairer interviews.</h1><p>Run consistent technical interviews with role-based workflows, evidence-led evaluation, and human oversight.</p>
        <div className="trust-list"><span><ShieldCheck /> Tenant-aware by design</span><span><UsersRound /> Human review required</span><span><Sparkles /> AI-assisted, never autonomous</span></div>
      </div>
      <p className="auth-footnote">{dataMode === "mock" ? "Mock environment · No real candidate data" : "Connected to protected API"}</p>
    </section>
    <section className="auth-panel"><div className="auth-mobile-brand"><Brand /></div><div className="auth-form-wrap"><p className="eyebrow">{eyebrow}</p><h2>{title}</h2><p className="muted">{description}</p>{children}</div></section>
  </main>;
}
