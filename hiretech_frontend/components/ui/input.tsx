import type { InputHTMLAttributes } from "react";
import { cn } from "@/components/ui/utils";

export function Input({ className, ...props }: InputHTMLAttributes<HTMLInputElement>) { return <input className={cn("input", className)} {...props} />; }

