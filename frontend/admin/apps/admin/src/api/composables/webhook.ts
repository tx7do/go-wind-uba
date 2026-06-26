import type {
  ubaservicev1_CountWebhookResponse,
  ubaservicev1_GetWebhookRequest,
  ubaservicev1_ListWebhookResponse,
  ubaservicev1_Webhook,
} from '#/generated/api/admin/service/v1';

import {
  useMutation,
  type UseMutationOptions,
  useQuery,
  type UseQueryOptions,
} from '@tanstack/vue-query';

import { apiClient } from '#/api/client';
import { queryClient } from '#/plugins/vue-query';
import { makeUpdateMask, type PaginationQuery } from '#/transport/rest';

// ==============================
// Webhook
// ==============================

export function useListWebhooks(
  query: PaginationQuery,
  options?: UseQueryOptions<ubaservicev1_ListWebhookResponse, Error>,
) {
  return useQuery({
    queryKey: ['listWebhooks', query],
    queryFn: () => apiClient.webhookService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListWebhooks(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listWebhooks', params],
    queryFn: () => apiClient.webhookService.List(params.toRawParams()),
    staleTime: 0,
    retry: 0,
  });
}

export function useCountWebhooks(
  query: PaginationQuery,
  options?: UseQueryOptions<ubaservicev1_CountWebhookResponse, Error>,
) {
  return useQuery({
    queryKey: ['countWebhooks', query],
    queryFn: () => apiClient.webhookService.Count(query.toRawParams()),
    ...options,
  });
}

export function useGetWebhook(
  req: ubaservicev1_GetWebhookRequest,
  options?: UseQueryOptions<ubaservicev1_Webhook, Error>,
) {
  return useQuery({
    queryKey: ['getWebhook', req],
    queryFn: () => apiClient.webhookService.Get(req),
    ...options,
  });
}

export function useCreateWebhook(
  options?: UseMutationOptions<
    ubaservicev1_Webhook,
    Error,
    Record<string, any>
  >,
) {
  return useMutation({
    mutationFn: (values) =>
      apiClient.webhookService.Create({ data: { ...values } as any }),
    ...options,
  });
}

export function useUpdateWebhook(
  options?: UseMutationOptions<
    ubaservicev1_Webhook,
    Error,
    { id: number; values: Record<string, any> }
  >,
) {
  return useMutation({
    mutationFn: ({ id, values }: { id: number; values: Record<string, any> }) =>
      apiClient.webhookService.Update({
        id,
        data: { ...values } as any,
        updateMask: makeUpdateMask(Object.keys(values ?? {})),
      }),
    ...options,
  });
}

export function useDeleteWebhook(
  options?: UseMutationOptions<object, Error, number>,
) {
  return useMutation({
    mutationFn: (id) => apiClient.webhookService.Delete({ id }),
    ...options,
  });
}
