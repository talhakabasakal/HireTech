import type { SelectHTMLAttributes } from "react";
import { cn } from "@/components/ui/utils";

export function Select({ className, ...props }: SelectHTMLAttributes<HTMLSelectElement>) { return <select className={cn("input select", className)} {...props} />; }

