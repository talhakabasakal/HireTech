"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { CheckCircle2, Circle, Headphones, Laptop, LoaderCircle, Wifi, WifiOff, XCircle } from "lucide-react";
import { WorkspaceShell } from "@/components/layout/workspace-shell";
import { PageHeader } from "@/components/layout/page-header";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useMockData } from "@/core/config/runtime";

type CheckState = "pending" | "checking" | "ready" | "failed";
interface EnvironmentCheck { id: "browser" | "connection" | "audio"; name: string; description: string; state: CheckState; icon: typeof Laptop }

function canUseStorage(): boolean { try { const key = "hiretech-preparation-check"; sessionStorage.setItem(key, "ok"); sessionStorage.removeItem(key); return true; } catch { return false; } }

function initialCheckStates(): Record<EnvironmentCheck["id"], CheckState> {
  if (useMockData) return { browser: "ready", connection: "ready", audio: "ready" };
  if (typeof window === "undefined") return { browser: "pending", connection: "pending", audio: "pending" };
  const browserReady = typeof crypto?.randomUUID === "function" && canUseStorage();
  return { browser: browserReady ? "ready" : "failed", connection: navigator.onLine ? "ready" : "failed", audio: "pending" };
}

export function PreparationView() {
  const checks = useMemo<EnvironmentCheck[]>(() => [
    { id: "browser", icon: Laptop, name: "Browser compatibility", description: "Secure storage and UUID support are available.", state: "pending" },
    { id: "connection", icon: Wifi, name: "Connection stability", description: "The browser can reach the network for answer sync.", state: "pending" },
    { id: "audio", icon: Headphones, name: "Audio permission", description: "Microphone access is checked only when you ask us to test it.", state: "pending" },
  ], []);
  const [checkStates, setCheckStates] = useState<Record<EnvironmentCheck["id"], CheckState>>(initialCheckStates);
  const [message, setMessage] = useState<string | null>(null);

  useEffect(() => {
    if (useMockData) return;
    const updateConnection = () => setCheckStates((current) => ({ ...current, connection: navigator.onLine ? "ready" : "failed" }));
    window.addEventListener("online", updateConnection);
    window.addEventListener("offline", updateConnection);
    return () => { window.removeEventListener("online", updateConnection); window.removeEventListener("offline", updateConnection); };
  }, []);

  const checkAudio = async () => {
    if (useMockData) return;
    setCheckStates((current) => ({ ...current, audio: "checking" }));
    setMessage(null);
    if (!navigator.mediaDevices?.getUserMedia) { setCheckStates((current) => ({ ...current, audio: "failed" })); setMessage("This browser does not provide microphone access."); return; }
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      stream.getTracks().forEach((track) => track.stop());
      setCheckStates((current) => ({ ...current, audio: "ready" }));
    } catch { setCheckStates((current) => ({ ...current, audio: "failed" })); setMessage("Microphone permission is required to enter the interview."); }
  };

  const failed = Object.values(checkStates).some((state) => state === "failed");
  const canEnter = Object.values(checkStates).every((state) => state === "ready");
  const stateLabel = (state: CheckState) => state === "ready" ? "Ready" : state === "checking" ? "Checking…" : state === "failed" ? "Failed" : "Check now";
  const stateIcon = (state: CheckState) => state === "ready" ? <CheckCircle2 aria-hidden="true" /> : state === "failed" ? <XCircle aria-hidden="true" /> : state === "checking" ? <LoaderCircle className="spin" aria-hidden="true" /> : <Circle aria-hidden="true" />;

  return <WorkspaceShell area="candidate" variant="candidate-portal"><main className="page narrow-page"><PageHeader eyebrow="Step 2 of 3" title="Prepare your environment" description="Complete these checks before entering the interview workspace." /><Card><CardHeader><CardTitle>System check</CardTitle></CardHeader><CardContent className="check-list">{checks.map(({ icon: Icon, id, name, description }) => { const state = checkStates[id]; return <div key={id} className="check-row"><Icon aria-hidden="true" /><div><strong>{name}</strong><span>{description}</span></div><span className={state === "ready" ? "status-ready" : state === "failed" ? "field-error" : "status-pending"}>{stateIcon(state)}{id === "audio" && state === "pending" && !useMockData ? <button className="text-button" type="button" onClick={() => void checkAudio()}>Check now</button> : stateLabel(state)}</span></div>; })}</CardContent></Card>{!useMockData && <p className="helper" role="note">No microphone recording is kept by this check. Access is released immediately after capability verification.</p>}{message && <p className="field-error" role="alert">{message}</p>}{failed && <div className="callout"><strong>Resolve the failed checks</strong><p>Reconnect or allow the required browser capability, then run the check again before entering.</p></div>}<div className="callout"><strong>Before you begin</strong><p>Find a quiet space, close unrelated applications, and keep your charger connected. Drafts stay in this browser until you explicitly sync them.</p></div><div className="page-actions"><Button asChild variant="outline"><Link href="/candidate/invitation">Back</Link></Button><Button asChild={canEnter}><span>{canEnter ? <Link href="/candidate/interview">Enter interview</Link> : <><WifiOff aria-hidden="true" /> Complete checks to enter</>}</span></Button></div></main></WorkspaceShell>;
}
