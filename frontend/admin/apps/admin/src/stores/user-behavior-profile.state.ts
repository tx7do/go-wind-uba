import { useUserStore } from '@vben/stores';

import { defineStore } from 'pinia';

import { createUserBehaviorProfileServiceClient } from '#/generated/api/admin/service/v1';
import { makeOrderBy, makeQueryString } from '#/utils/query';
import { type Paging, requestClientRequestHandler } from '#/utils/request';

export const useUserBehaviorProfileListStore = defineStore(
  'user-behavior-profile-list',
  () => {
    const service = createUserBehaviorProfileServiceClient(
      requestClientRequestHandler,
    );
    const userStore = useUserStore();

    /**
     * 查询用户画像列表
     */
    async function listUserBehaviorProfile(
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
     * 获取用户画像
     */
    async function getUserBehaviorProfile(id: number) {
      return await service.Get({ id });
    }

    function $reset() {}

    return {
      $reset,
      listUserBehaviorProfile,
      getUserBehaviorProfile,
    };
  },
);
