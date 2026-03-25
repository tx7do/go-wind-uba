import { computed } from 'vue';

import { $t } from '@vben/locales';
import { useUserStore } from '@vben/stores';

import { defineStore } from 'pinia';

import {
  createTagDefinitionServiceClient,
  type ubaservicev1_TagCategory as TagCategory,
  type ubaservicev1_TagType as TagType,
} from '#/generated/api/admin/service/v1';
import { makeOrderBy, makeQueryString, makeUpdateMask } from '#/utils/query';
import { type Paging, requestClientRequestHandler } from '#/utils/request';

export const useTagDefinitionListStore = defineStore(
  'tag-definition-list',
  () => {
    const service = createTagDefinitionServiceClient(
      requestClientRequestHandler,
    );
    const userStore = useUserStore();

    /**
     * 查询标签定义列表
     */
    async function listTagDefinition(
      paging?: Paging,
      formValues?: null | object,
      fieldMask?: null | string,
      orderBy?: null | string[],
    ) {
      const noPaging =
        paging?.page === undefined && paging?.pageSize === undefined;
      return await service.List({
        // @ts-ignore proto generated code is error.
        fieldMask,
        orderBy: makeOrderBy(orderBy),
        query: makeQueryString(formValues, userStore.isTenantUser()),
        page: paging?.page,
        pageSize: paging?.pageSize,
        noPaging,
      });
    }

    /**
     * 获取标签定义
     */
    async function getTagDefinition(id: number) {
      return await service.Get({ id });
    }

    /**
     * 创建标签定义
     */
    async function createTagDefinition(values: Record<string, any> = {}) {
      return await service.Create({
        // @ts-ignore proto generated code is error.
        data: {
          ...values,
        },
      });
    }

    /**
     * 更新标签定义
     */
    async function updateTagDefinition(
      id: number,
      values: Record<string, any> = {},
    ) {
      if ('id' in values) delete values.id;

      return await service.Update({
        id,
        // @ts-ignore proto generated code is error.
        data: {
          ...values,
        },
        // @ts-ignore proto generated code is error.
        updateMask: makeUpdateMask(Object.keys(values ?? [])),
      });
    }

    /**
     * 删除标签定义
     */
    async function deleteTagDefinition(id: number) {
      return await service.Delete({ id });
    }

    function $reset() {}

    return {
      $reset,
      listTagDefinition,
      getTagDefinition,
      createTagDefinition,
      updateTagDefinition,
      deleteTagDefinition,
    };
  },
);

export const tagCategoryList = computed(() => [
  {
    value: 'TAG_CATEGORY_USER',
    label: $t('enum.tagDefinition.category.TAG_CATEGORY_USER'),
  },
  {
    value: 'TAG_CATEGORY_BEHAVIOR',
    label: $t('enum.tagDefinition.category.TAG_CATEGORY_BEHAVIOR'),
  },
  {
    value: 'TAG_CATEGORY_RISK',
    label: $t('enum.tagDefinition.category.TAG_CATEGORY_RISK'),
  },
  {
    value: 'TAG_CATEGORY_BUSINESS',
    label: $t('enum.tagDefinition.category.TAG_CATEGORY_BUSINESS'),
  },
]);

const TAG_CATEGORY_COLOR_MAP = {
  TAG_CATEGORY_UNSPECIFIED: '#86909C',
  TAG_CATEGORY_USER: '#4096FF',
  TAG_CATEGORY_BEHAVIOR: '#00B42A',
  TAG_CATEGORY_RISK: '#F53F3F',
  TAG_CATEGORY_BUSINESS: '#722ED1',
  DEFAULT: '#86909C',
} as const;

export function tagCategoryToColor(category?: TagCategory) {
  return (
    TAG_CATEGORY_COLOR_MAP[category as keyof typeof TAG_CATEGORY_COLOR_MAP] ||
    TAG_CATEGORY_COLOR_MAP.DEFAULT
  );
}

export function tagCategoryToName(category?: TagCategory) {
  const values = tagCategoryList.value;
  const matchedItem = values.find((item) => item.value === category);
  return matchedItem ? matchedItem.label : '';
}

export const tagTypeList = computed(() => [
  {
    value: 'TAG_TYPE_BOOLEAN',
    label: $t('enum.tagDefinition.type.TAG_TYPE_BOOLEAN'),
  },
  {
    value: 'TAG_TYPE_ENUM',
    label: $t('enum.tagDefinition.type.TAG_TYPE_ENUM'),
  },
  {
    value: 'TAG_TYPE_NUMERIC',
    label: $t('enum.tagDefinition.type.TAG_TYPE_NUMERIC'),
  },
  {
    value: 'TAG_TYPE_STRING',
    label: $t('enum.tagDefinition.type.TAG_TYPE_STRING'),
  },
  {
    value: 'TAG_TYPE_LIST',
    label: $t('enum.tagDefinition.type.TAG_TYPE_LIST'),
  },
]);

const TAG_TYPE_COLOR_MAP = {
  TAG_TYPE_UNSPECIFIED: '#86909C',
  TAG_TYPE_BOOLEAN: '#4096FF',
  TAG_TYPE_ENUM: '#00B42A',
  TAG_TYPE_NUMERIC: '#F77234',
  TAG_TYPE_STRING: '#722ED1',
  TAG_TYPE_LIST: '#FF9A2E',
  DEFAULT: '#86909C',
} as const;

export function tagTypeToColor(type?: TagType) {
  return (
    TAG_TYPE_COLOR_MAP[type as keyof typeof TAG_TYPE_COLOR_MAP] ||
    TAG_TYPE_COLOR_MAP.DEFAULT
  );
}

export function tagTypeToName(type?: TagType) {
  const values = tagTypeList.value;
  const matchedItem = values.find((item) => item.value === type);
  return matchedItem ? matchedItem.label : '';
}
