import { ApplicationError } from "@/core/errors/application-error";
import { apiRequest } from "@/core/infrastructure/api/http-client";

interface GraphQLError { message: string; extensions?: { code?: string } }
interface GraphQLResponse<T> { data?: T; errors?: GraphQLError[] }
interface GraphQLRequestOptions { authenticated?: boolean }

export async function graphqlRequest<TData, TVariables extends Record<string, unknown> = Record<string, never>>(
  query: string,
  variables?: TVariables,
  options: GraphQLRequestOptions = {},
): Promise<TData> {
  const response = await apiRequest<GraphQLResponse<TData>>("/graphql", {
    method: "POST",
    body: JSON.stringify({ query, variables: variables ?? {} }),
  }, options.authenticated ?? true);
  const firstError = response.errors?.[0];
  if (firstError) throw new ApplicationError(firstError.message, firstError.extensions?.code ?? "GRAPHQL_ERROR");
  if (!response.data) throw new ApplicationError("GraphQL yanıtında veri bulunamadı.", "EMPTY_GRAPHQL_RESPONSE");
  return response.data;
}
