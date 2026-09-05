import type { HTMLAttributes } from "react";
import { cn } from "@/components/ui/utils";

export function Alert({ className, ...props }: HTMLAttributes<HTMLDivElement>) { return <div className={cn("alert", className)} role="status" {...props} />; }

