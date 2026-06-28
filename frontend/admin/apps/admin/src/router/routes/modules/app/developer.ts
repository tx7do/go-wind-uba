import type { RouteRecordRaw } from 'vue-router';

import { BasicLayout } from '#/layouts';
import { $t } from '#/locales';

// 开发者（从 system 拆出：API/字典/任务/语言/事件Schema）
const developer: RouteRecordRaw[] = [
  {
    path: '/developer',
    name: 'Developer',
    component: BasicLayout,
    redirect: '/developer/apis',
    meta: {
      order: 3000,
      icon: 'lucide:terminal',
      title: $t('menu.developer.moduleName'),
      keepAlive: true,
      authority: ['sys:platform_admin'],
    },
    children: [
      {
        path: 'apis',
        name: 'APIManagement',
        meta: {
          order: 1,
          icon: 'lucide:route',
          title: $t('menu.developer.api'),
          authority: ['sys:platform_admin'],
        },
        component: () => import('#/views/app/system/api/index.vue'),
      },

      {
        path: 'dict',
        name: 'DictManagement',
        meta: {
          order: 2,
          icon: 'lucide:library-big',
          title: $t('menu.developer.dict'),
          authority: ['sys:platform_admin'],
        },
        component: () => import('#/views/app/system/dict/index.vue'),
      },

      {
        path: 'tasks',
        name: 'TaskManagement',
        meta: {
          order: 3,
          icon: 'lucide:list-todo',
          title: $t('menu.developer.task'),
          authority: ['sys:platform_admin', 'sys:tenant_manager'],
        },
        component: () => import('#/views/app/system/task/index.vue'),
      },

      {
        path: 'languages',
        name: 'LanguageManagement',
        meta: {
          order: 4,
          icon: 'lucide:globe',
          title: $t('menu.developer.language'),
          authority: ['sys:platform_admin'],
        },
        component: () => import('#/views/app/system/language/index.vue'),
      },

      {
        path: 'event-schemas',
        name: 'EventSchemaManagement',
        meta: {
          order: 5,
          icon: 'lucide:braces',
          title: $t('menu.developer.eventSchema'),
          authority: ['sys:platform_admin'],
        },
        component: () => import('#/views/app/system/event-schema/index.vue'),
      },
    ],
  },
];

export default developer;
