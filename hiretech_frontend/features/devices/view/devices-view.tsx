"use client";

import { AlertTriangle, Laptop, MapPin, ShieldCheck, Smartphone } from "lucide-react";
import { WorkspaceShell } from "@/components/layout/workspace-shell";
import { PageHeader } from "@/components/layout/page-header";
import { EmptyState, ErrorState, LoadingState, SuccessState } from "@/components/states/async-state";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { useDeviceViewModel } from "@/features/devices/view-model/use-device-view-model";

export function DevicesView() {
  const vm = useDeviceViewModel();
  return <WorkspaceShell area="candidate"><main className="page"><PageHeader eyebrow="Account security" title="Registered devices" description="Review devices with access to this account and revoke anything you do not recognize." />{vm.state.status === "loading" && vm.state.devices.length === 0 && <LoadingState label="Loading devices" />}{vm.state.status === "error" && vm.state.devices.length === 0 && <ErrorState message={vm.state.message ?? "Unable to load devices."} onRetry={() => void vm.load()} />}{vm.state.message && vm.state.devices.length > 0 && <SuccessState message={vm.state.message} />}{vm.state.devices.length === 0 && vm.state.status === "success" && <EmptyState title="No registered devices" description="Devices will appear after a verified sign-in." />}<section className="device-list">{vm.state.devices.map((device) => <Card key={device.id} className={device.trust === "suspicious" ? "device-card suspicious" : "device-card"}><CardContent><span className="device-icon">{device.name.includes("phone") ? <Smartphone /> : <Laptop />}</span><div className="device-copy"><div><h2>{device.name}</h2><Badge>{device.trust}</Badge></div><p>{device.browser}</p><span><MapPin /> {device.location} · Last seen {device.trust === "current" ? "now" : "recently"}</span></div>{device.trust === "current" ? <span className="current-device"><ShieldCheck /> Current device</span> : <Button variant={device.trust === "suspicious" ? "danger" : "outline"} onClick={() => void vm.revoke(device.id)} disabled={vm.state.status === "loading"}>{device.trust === "suspicious" && <AlertTriangle />} Revoke</Button>}</CardContent></Card>)}</section><div className="callout"><strong>Suspicious activity</strong><p>Revoking a device should invalidate its backend refresh token and create an immutable audit event. This mock only updates in-memory state.</p></div></main></WorkspaceShell>;
}

