import type { RouteRecordRaw } from 'vue-router';

import { BasicLayout } from '#/layouts';
import { $t } from '#/locales';

const ubaApplication: RouteRecordRaw[] = [
  {
    path: '/app',
    name: 'ApplicationManagement',
    component: BasicLayout,
    redirect: '/app/applications',
    meta: {
      order: 100,
      icon: 'lucide:square-stack',
      title: $t('menu.application.moduleName'),
      authority: ['sys:platform_admin'],
    },
    children: [
      {
        path: 'applications',
        name: 'UbaApplicationList',
        meta: {
          order: 1,
          icon: 'lucide:box',
          title: $t('menu.application.applications'),
          authority: ['sys:platform_admin'],
        },
        component: () =>
          import('#/views/app/application/application/index.vue'),
      },
    ],
  },
];

export default ubaApplication;
