import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
import type { ButtonHTMLAttributes } from "react";
import { cn } from "@/components/ui/utils";

const buttonVariants = cva("button", {
  variants: {
    variant: { default: "button-primary", secondary: "button-secondary", outline: "button-outline", ghost: "button-ghost", danger: "button-danger" },
    size: { default: "button-md", sm: "button-sm", lg: "button-lg", icon: "button-icon" },
  },
  defaultVariants: { variant: "default", size: "default" },
});

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement>, VariantProps<typeof buttonVariants> { asChild?: boolean }

export function Button({ asChild = false, className, variant, size, ...props }: ButtonProps) {
  const Component = asChild ? Slot : "button";
  return <Component className={cn(buttonVariants({ variant, size }), className)} {...props} />;
}

