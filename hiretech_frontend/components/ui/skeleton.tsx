import { cn } from "@/components/ui/utils";
export function Skeleton({ className }: { className?: string }) { return <div className={cn("skeleton", className)} aria-hidden="true" />; }

