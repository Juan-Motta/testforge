import { z } from "zod";

// Mirrors `SignupRequest` / `LoginRequest` in backend/api/openapi.yaml.
export const signupSchema = z.object({
  name: z
    .string()
    .trim()
    .min(1, "Enter your name.")
    .max(100, "Use 100 characters or fewer."),
  email: z
    .email("Enter a valid email address.")
    .max(254, "Use 254 characters or fewer."),
  password: z
    .string()
    .min(8, "Use at least 8 characters.")
    .max(256, "Use 256 characters or fewer."),
});

export const loginSchema = z.object({
  email: z.email("Enter a valid email address."),
  password: z.string().min(1, "Enter your password."),
});

export type SignupValues = z.infer<typeof signupSchema>;
export type LoginValues = z.infer<typeof loginSchema>;
