import { z } from "zod";

export const loginSchema = z.object({
  email: z.email("Enter a valid email address."),
  password: z.string().min(8, "Password must contain at least 8 characters."),
});

export const registerSchema = loginSchema.extend({ name: z.string().trim().min(2, "Enter your full name.") });
export const emailSchema = z.object({ email: z.email("Enter a valid email address.") });
export const verificationSchema = emailSchema.extend({ code: z.string().regex(/^\d{6}$/, "Enter the complete six-digit code.") });

