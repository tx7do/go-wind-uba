import type {
  resourceservicev1_DeleteMenuRequest,
  resourceservicev1_GetMenuRequest,
  resourceservicev1_ListMenuResponse,
  resourceservicev1_Menu,
  resourceservicev1_Menu_Type as Menu_Type,
} from '#/generated/api/admin/service/v1';

import { computed } from 'vue';

import { $t } from '@vben/locales';

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
// 菜单管理
// ==============================

export function useListMenus(
  query: PaginationQuery,
  options?: UseQueryOptions<resourceservicev1_ListMenuResponse, Error>,
) {
  return useQuery({
    queryKey: ['listMenus', query],
    queryFn: () => apiClient.menuService.List(query.toRawParams()),
    ...options,
  });
}

export async function fetchListMenus(params: PaginationQuery) {
  return queryClient.fetchQuery({
    queryKey: ['listMenus', params],
    queryFn: () => apiClient.menuService.List(params.toRawParams()),
    staleTime: 0,
    retry: 0,
  });
}

export function useGetMenu(
  req: resourceservicev1_GetMenuRequest,
  options?: UseQueryOptions<resourceservicev1_Menu, Error>,
) {
  return useQuery({
    queryKey: ['getMenu', req],
    queryFn: () => apiClient.menuService.Get(req),
    ...options,
  });
}

export function useCreateMenu(
  options?: UseMutationOptions<object, Error, Record<string, any>>,
) {
  return useMutation({
    mutationFn: (values) =>
      apiClient.menuService.Create({ data: { ...values } as resourceservicev1_Menu }),
    ...options,
  });
}

export function useUpdateMenu(
  options?: UseMutationOptions<
    object,
    Error,
    { id: number; values: Record<string, any> }
  >,
) {
  return useMutation({
    mutationFn: ({ id, values }: { id: number; values: Record<string, any> }) =>
      apiClient.menuService.Update({
        id,
        data: { ...values } as any,
        updateMask: makeUpdateMask(Object.keys(values ?? {})),
      }),
    ...options,
  });
}

export function useDeleteMenu(
  options?: UseMutationOptions<
    object,
    Error,
    resourceservicev1_DeleteMenuRequest
  >,
) {
  return useMutation({
    mutationFn: (data) => apiClient.menuService.Delete(data),
    ...options,
  });
}

// ==============================
// 菜单枚举与工具函数
// ==============================

export const menuTypeList = computed(() => [
  { value: 'CATALOG', label: $t('enum.menu.type.CATALOG') },
  { value: 'MENU', label: $t('enum.menu.type.MENU') },
  { value: 'BUTTON', label: $t('enum.menu.type.BUTTON') },
  { value: 'EMBEDDED', label: $t('enum.menu.type.EMBEDDED') },
  { value: 'LINK', label: $t('enum.menu.type.LINK') },
]);

export function menuTypeToName(menuType: any): string {
  const values = menuTypeList.value;
  const matchedItem = values.find((item) => item.value === menuType);
  return matchedItem ? matchedItem.label : menuType;
}

export function menuTypeToColor(menuType: Menu_Type) {
  switch (menuType) {
    case 'BUTTON': {
      return '#F56C6C';
    }
    case 'CATALOG': {
      return '#27AE60';
    }
    case 'EMBEDDED': {
      return '#4096FF';
    }
    case 'LINK': {
      return '#9B59B6';
    }
    case 'MENU': {
      return '#165DFF';
    }
    default: {
      return '#86909C';
    }
  }
}

export const isCatalog = (type: string) => type === 'CATALOG';
export const isMenu = (type: string) => type === 'MENU';
export const isButton = (type: string) => type === 'BUTTON';
export const isEmbedded = (type: string) => type === 'EMBEDDED';
export const isLink = (type: string) => type === 'LINK';

/** 遍历菜单子节点 */
export function travelMenuChild(
  nodes: resourceservicev1_Menu[] | undefined,
  parent: resourceservicev1_Menu,
): boolean {
  if (nodes === undefined) return false;
  if (parent.parentId === 0 || parent.parentId === undefined) {
    if (parent?.meta?.title) {
      parent.meta.title = $t(parent?.meta?.title ?? '');
    }
    nodes.push(parent);
    return true;
  }
  for (const node of nodes) {
    if (node === undefined) continue;
    if (node.id === parent.parentId) {
      if (parent?.meta?.title) {
        parent.meta.title = $t(parent?.meta?.title ?? '');
      }
      if (node.children !== undefined) node.children.push(parent);
      return true;
    }
    if (travelMenuChild(node.children, parent)) return true;
  }
  return false;
}

/**
 * 构建菜单树
 */
export function buildMenuTree(menus: resourceservicev1_Menu[]): resourceservicev1_Menu[] {
  // 深拷贝，避免修改缓存中的原始数据
  const cloned = structuredClone(menus);
  const tree: resourceservicev1_Menu[] = [];
  for (const menu of cloned) {
    if (!menu) continue;
    if (menu.parentId !== 0 && menu.parentId !== undefined) continue;
    if (menu?.meta?.title) {
      menu.meta.title = $t(menu?.meta?.title ?? '');
    }
    tree.push(menu);
  }
  for (const menu of cloned) {
    if (!menu) continue;
    if (menu.parentId === 0 || menu.parentId === undefined) continue;
    if (travelMenuChild(tree, menu)) continue;
    if (menu?.meta?.title) {
      menu.meta.title = $t(menu?.meta?.title ?? '');
    }
    tree.push(menu);
  }
  return tree;
}
