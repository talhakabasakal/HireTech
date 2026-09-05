import type { TextareaHTMLAttributes } from "react";
import { cn } from "@/components/ui/utils";

export function Textarea({ className, ...props }: TextareaHTMLAttributes<HTMLTextAreaElement>) { return <textarea className={cn("textarea", className)} {...props} />; }

