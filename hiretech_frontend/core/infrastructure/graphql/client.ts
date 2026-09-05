import { ApplicationError } from "@/core/errors/application-error";
import { apiRequest } from "@/core/infrastructure/api/http-client";

interface GraphQLError { message: string; extensions?: { code?: string } }
interface GraphQLResponse<T> { data?: T; errors?: GraphQLError[] }
interface GraphQLRequestOptions { authenticated?: boolean }

const persistedOperationsEnabled = process.env.NEXT_PUBLIC_GRAPHQL_PERSISTED_OPERATIONS === "true";

async function sha256Hex(value: string) {
  if (!globalThis.crypto?.subtle) throw new ApplicationError("Persisted GraphQL operation hashing is unavailable.", "GRAPHQL_HASH_UNAVAILABLE");
  const digest = await globalThis.crypto.subtle.digest("SHA-256", new TextEncoder().encode(value));
  return Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, "0")).join("");
}

export async function graphqlRequest<TData, TVariables extends Record<string, unknown> = Record<string, never>>(
  query: string,
  variables?: TVariables,
  options: GraphQLRequestOptions = {},
): Promise<TData> {
  const body: { query: string; variables: TVariables; extensions?: { persistedQuery: { version: 1; sha256Hash: string } } } = { query, variables: variables ?? {} as TVariables };
  if (persistedOperationsEnabled) {
    body.extensions = { persistedQuery: { version: 1, sha256Hash: await sha256Hex(query) } };
  }
  const response = await apiRequest<GraphQLResponse<TData>>("/graphql", {
    method: "POST",
    body: JSON.stringify(body),
  }, options.authenticated ?? true);
  const firstError = response.errors?.[0];
  if (firstError) throw new ApplicationError(firstError.message, firstError.extensions?.code ?? "GRAPHQL_ERROR");
  if (!response.data) throw new ApplicationError("GraphQL yanıtında veri bulunamadı.", "EMPTY_GRAPHQL_RESPONSE");
  return response.data;
}
