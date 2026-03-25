import type { RouteRecordRaw } from 'vue-router';

import { BasicLayout } from '#/layouts';
import { $t } from '#/locales';

const object: RouteRecordRaw[] = [
  {
    path: '/object',
    name: 'ObjectManagement',
    component: BasicLayout,
    redirect: '/object/objects',
    meta: {
      order: 500,
      icon: 'lucide:layers',
      title: $t('menu.object.moduleName'),
      authority: ['sys:platform_admin'],
    },
    children: [
      {
        path: 'objects',
        name: 'ObjectList',
        meta: {
          order: 1,
          icon: 'lucide:box',
          title: $t('menu.object.objects'),
          authority: ['sys:platform_admin'],
        },
        component: () => import('#/views/app/object/object/index.vue'),
      },
    ],
  },
];

export default object;
