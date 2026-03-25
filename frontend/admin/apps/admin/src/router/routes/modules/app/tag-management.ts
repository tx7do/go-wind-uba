import type { RouteRecordRaw } from 'vue-router';

import { BasicLayout } from '#/layouts';
import { $t } from '#/locales';

const tag: RouteRecordRaw[] = [
  {
    path: '/tag',
    name: 'TagManagement',
    component: BasicLayout,
    redirect: '/tag/tags',
    meta: {
      order: 400,
      icon: 'lucide:tags',
      title: $t('menu.tag.moduleName'),
      authority: ['sys:platform_admin'],
    },
    children: [
      {
        path: 'tags',
        name: 'TagDefinitionManagement',
        meta: {
          order: 1,
          icon: 'lucide:badge-check',
          title: $t('menu.tag.tags'),
          authority: ['sys:platform_admin'],
        },
        component: () => import('#/views/app/tag/tags/index.vue'),
      },
      {
        path: 'user-tags',
        name: 'UserTagManagement',
        meta: {
          order: 2,
          icon: 'lucide:bookmark',
          title: $t('menu.tag.userTags'),
          authority: ['sys:platform_admin'],
        },
        component: () => import('#/views/app/tag/user-tags/index.vue'),
      },
      {
        path: 'ids',
        name: 'IDMappingManagement',
        meta: {
          order: 3,
          icon: 'lucide:link',
          title: $t('menu.tag.ids'),
          authority: ['sys:platform_admin'],
        },
        component: () => import('#/views/app/tag/ids/index.vue'),
      },
    ],
  },
];

export default tag;
