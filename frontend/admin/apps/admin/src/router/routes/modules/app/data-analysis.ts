import type { RouteRecordRaw } from 'vue-router';

import { BasicLayout } from '#/layouts';
import { $t } from '#/locales';

const dataAnalysis: RouteRecordRaw[] = [
  {
    path: '/data-analysis',
    name: 'DataAnalysis',
    component: BasicLayout,
    redirect: '/data-analysis/profile',
    meta: {
      order: 200,
      icon: 'lucide:chart-bar',
      title: $t('menu.dataAnalysis.moduleName'),
      authority: ['sys:platform_admin'],
    },
    children: [
      {
        path: 'sessions',
        name: 'SessionManagement',
        meta: {
          order: 1,
          icon: 'lucide:clock',
          title: $t('menu.dataAnalysis.session'),
          authority: ['sys:platform_admin'],
        },
        component: () => import('#/views/app/data-analysis/session/index.vue'),
      },
      {
        path: 'event-paths',
        name: 'EventPathManagement',
        meta: {
          order: 2,
          icon: 'lucide:route',
          title: $t('menu.dataAnalysis.eventPath'),
          authority: ['sys:platform_admin'],
        },
        component: () =>
          import('#/views/app/data-analysis/event-path/index.vue'),
      },
      {
        path: 'profile',
        name: 'UserBehaviorProfile',
        meta: {
          order: 3,
          icon: 'lucide:user-check',
          title: $t('menu.dataAnalysis.userBehaviorProfile'),
          authority: ['sys:platform_admin'],
        },
        component: () => import('#/views/app/data-analysis/profile/index.vue'),
      },
    ],
  },
];

export default dataAnalysis;
