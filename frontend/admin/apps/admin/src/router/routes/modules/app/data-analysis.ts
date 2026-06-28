import type { RouteRecordRaw } from 'vue-router';

import { BasicLayout } from '#/layouts';
import { $t } from '#/locales';

const dataAnalysis: RouteRecordRaw[] = [
  {
    path: '/data-analysis',
    name: 'DataAnalysis',
    component: BasicLayout,
    redirect: '/data-analysis/event-trend',
    meta: {
      order: 1000,
      icon: 'lucide:chart-bar',
      title: $t('menu.dataAnalysis.moduleName'),
      authority: ['sys:platform_admin'],
    },
    children: [
      {
        path: 'event-trend',
        name: 'EventTrendAnalysis',
        meta: {
          order: 1,
          icon: 'lucide:trending-up',
          title: $t('menu.dataAnalysis.eventTrend'),
          authority: ['sys:platform_admin'],
        },
        component: () =>
          import('#/views/app/data-analysis/event-trend/index.vue'),
      },
      {
        path: 'funnel',
        name: 'FunnelAnalysis',
        meta: {
          order: 2,
          icon: 'lucide:filter',
          title: $t('menu.dataAnalysis.funnel'),
          authority: ['sys:platform_admin'],
        },
        component: () => import('#/views/app/data-analysis/funnel/index.vue'),
      },
      {
        path: 'retention',
        name: 'RetentionAnalysis',
        meta: {
          order: 3,
          icon: 'lucide:repeat',
          title: $t('menu.dataAnalysis.retention'),
          authority: ['sys:platform_admin'],
        },
        component: () =>
          import('#/views/app/data-analysis/retention/index.vue'),
      },
      {
        path: 'dimension-compare',
        name: 'DimensionCompare',
        meta: {
          order: 4,
          icon: 'lucide:bar-chart-3',
          title: $t('menu.dataAnalysis.dimensionCompare'),
          authority: ['sys:platform_admin'],
        },
        component: () =>
          import('#/views/app/data-analysis/dimension-compare/index.vue'),
      },
      {
        path: 'behavior-timeline',
        name: 'BehaviorTimeline',
        meta: {
          order: 5,
          icon: 'lucide:list-todo',
          title: $t('menu.dataAnalysis.behaviorTimeline'),
          authority: ['sys:platform_admin'],
        },
        component: () =>
          import('#/views/app/data-analysis/behavior-timeline/index.vue'),
      },
      {
        path: 'realtime-screen',
        name: 'RealtimeScreen',
        meta: {
          order: 6,
          icon: 'lucide:radio',
          title: $t('menu.dataAnalysis.realtimeScreen'),
          authority: ['sys:platform_admin'],
        },
        component: () =>
          import('#/views/app/data-analysis/realtime-screen/index.vue'),
      },
      {
        path: 'sessions',
        name: 'SessionManagement',
        meta: {
          order: 7,
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
          order: 8,
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
          order: 9,
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
