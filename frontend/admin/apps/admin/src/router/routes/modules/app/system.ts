import type { RouteRecordRaw } from 'vue-router';

import { BasicLayout } from '#/layouts';
import { $t } from '#/locales';

// 系统管理（保留核心系统配置：菜单/文件/登录策略）
// 开发者相关（API/字典/任务/语言/事件Schema）已拆分至 developer.ts
const system: RouteRecordRaw[] = [
  {
    path: '/system',
    name: 'System',
    component: BasicLayout,
    redirect: '/system/menus',
    meta: {
      order: 2000,
      icon: 'lucide:settings',
      title: $t('menu.system.moduleName'),
      keepAlive: true,
      authority: ['sys:platform_admin', 'sys:tenant_manager'],
    },
    children: [
      {
        path: 'menus',
        name: 'MenuManagement',
        meta: {
          order: 1,
          icon: 'lucide:square-menu',
          title: $t('menu.system.menu'),
          authority: ['sys:platform_admin'],
        },
        component: () => import('#/views/app/system/menu/index.vue'),
      },

      {
        path: 'files',
        name: 'FileManagement',
        meta: {
          order: 2,
          icon: 'lucide:file-search',
          title: $t('menu.system.file'),
          authority: ['sys:platform_admin', 'sys:tenant_manager'],
        },
        component: () => import('#/views/app/system/file/index.vue'),
      },

      {
        path: 'login-policies',
        name: 'LoginPolicyManagement',
        meta: {
          order: 3,
          icon: 'lucide:shield-x',
          title: $t('menu.system.loginPolicy'),
          authority: ['sys:platform_admin'],
        },
        component: () => import('#/views/app/system/login_policy/index.vue'),
      },
    ],
  },
];

export default system;
