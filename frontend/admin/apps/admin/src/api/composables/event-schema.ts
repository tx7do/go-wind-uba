import type {
  ubaservicev1_CountEventSchemaResponse,
  ubaservicev1_DeleteEventSchemaRequest,
  ubaservicev1_EventSchema,
  ubaservicev1_GetEventSchemaRequest,
  ubaservicev1_ListEventSchemaResponse,
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
// 事件 Schema 管理
// ==============================
export function useListEventSchemas(
  query: PaginationQuery,
  options?: UseQueryOptions<ubaservicev1_ListEventSchemaResponse, Error>,
) {
  return useQuery({
    queryKey: ['listEventSchemas', query],
    queryFn: () => apiClient.eventSchemaService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListEventSchemas(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listEventSchemas', params],
    queryFn: () => apiClient.eventSchemaService.List(params.toRawParams()),
    staleTime: 0,
    retry: 0,
  });
}

export function useGetEventSchema(
  req: ubaservicev1_GetEventSchemaRequest,
  options?: UseQueryOptions<ubaservicev1_EventSchema, Error>,
) {
  return useQuery({
    queryKey: ['getEventSchema', req],
    queryFn: () => apiClient.eventSchemaService.Get(req),
    ...options,
  });
}

export function useCountEventSchemas(
  query: PaginationQuery,
  options?: UseQueryOptions<ubaservicev1_CountEventSchemaResponse, Error>,
) {
  return useQuery({
    queryKey: ['countEventSchemas', query],
    queryFn: () => apiClient.eventSchemaService.Count(query.toRawParams()),
    ...options,
  });
}

export function useCreateEventSchema(
  options?: UseMutationOptions<
    ubaservicev1_EventSchema,
    Error,
    ubaservicev1_EventSchema
  >,
) {
  return useMutation({
    mutationFn: (data) => apiClient.eventSchemaService.Create({ data }),
    ...options,
  });
}

export function useUpdateEventSchema(
  options?: UseMutationOptions<
    ubaservicev1_EventSchema,
    Error,
    { id: number; values: ubaservicev1_EventSchema }
  >,
) {
  return useMutation({
    mutationFn: ({
      id,
      values,
    }: {
      id: number;
      values: ubaservicev1_EventSchema;
    }) =>
      apiClient.eventSchemaService.Update({
        id,
        data: { ...values },
        updateMask: makeUpdateMask(Object.keys(values ?? {})),
      }),
    ...options,
  });
}

export function useDeleteEventSchema(
  options?: UseMutationOptions<
    object,
    Error,
    ubaservicev1_DeleteEventSchemaRequest
  >,
) {
  return useMutation({
    mutationFn: (data) => apiClient.eventSchemaService.Delete(data),
    ...options,
  });
}

// ==============================
// 枚举与工具
// ==============================
export const eventSchemaStatusList = [
  { value: 'ENABLED', label: '启用', color: '#00B42A' },
  { value: 'DISABLED', label: '停用', color: '#C9CDD4' },
];

export function eventSchemaStatusToName(value?: string): string {
  return eventSchemaStatusList.find((i) => i.value === value)?.label ?? '';
}

export function eventSchemaStatusToColor(value?: string): string {
  return (
    eventSchemaStatusList.find((i) => i.value === value)?.color ?? '#E5E7EB'
  );
}

export const eventPropertyTypeList = [
  'string',
  'int',
  'double',
  'bool',
  'array',
  'object',
];
