import type { RouteRecordRaw } from 'vue-router';

import { BasicLayout } from '#/layouts';
import { $t } from '#/locales';

const tenant: RouteRecordRaw[] = [
  {
    path: '/risk',
    name: 'RiskManagement',
    component: BasicLayout,
    redirect: '/risk/events',
    meta: {
      order: 300,
      icon: 'lucide:shield-alert',
      title: $t('menu.risk.moduleName'),
      authority: ['sys:platform_admin'],
    },
    children: [
      {
        path: 'events',
        name: 'RiskEventManagement',
        meta: {
          order: 1,
          icon: 'lucide:triangle-alert',
          title: $t('menu.risk.event'),
          authority: ['sys:platform_admin'],
        },
        component: () => import('#/views/app/risk/event/index.vue'),
      },
      {
        path: 'rules',
        name: 'RiskRuleManagement',
        meta: {
          order: 2,
          icon: 'lucide:file-check',
          title: $t('menu.risk.rule'),
          authority: ['sys:platform_admin'],
        },
        component: () => import('#/views/app/risk/rule/index.vue'),
      },
      {
        path: 'webhooks',
        name: 'WebhookManagement',
        meta: {
          order: 3,
          icon: 'lucide:webhook',
          title: $t('menu.risk.webhook'),
          authority: ['sys:platform_admin'],
        },
        component: () => import('#/views/app/risk/webhook/index.vue'),
      },
    ],
  },
];

export default tenant;
