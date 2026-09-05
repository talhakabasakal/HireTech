import type { LabelHTMLAttributes } from "react";
import { cn } from "@/components/ui/utils";

export function Label({ className, ...props }: LabelHTMLAttributes<HTMLLabelElement>) { return <label className={cn("label", className)} {...props} />; }

