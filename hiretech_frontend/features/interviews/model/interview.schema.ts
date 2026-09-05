import { z } from "zod";

export const answerSchema = z.object({
  response: z.string(),
  code: z.string(),
}).refine(({ response, code }) => response.trim().length >= 20 || code.trim().length > 0, {
  message: "Add a written answer or code before syncing this question.",
});

