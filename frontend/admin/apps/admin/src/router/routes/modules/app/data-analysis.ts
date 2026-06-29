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
      authority: ['sys:platform_admin', 'sys:tenant_manager'],
    },
    children: [
      {
        path: 'event-trend',
        name: 'EventTrendAnalysis',
        meta: {
          order: 1,
          icon: 'lucide:trending-up',
          title: $t('menu.dataAnalysis.eventTrend'),
          authority: ['sys:platform_admin', 'sys:tenant_manager'],
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
          authority: ['sys:platform_admin', 'sys:tenant_manager'],
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
          authority: ['sys:platform_admin', 'sys:tenant_manager'],
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
          authority: ['sys:platform_admin', 'sys:tenant_manager'],
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
          authority: ['sys:platform_admin', 'sys:tenant_manager'],
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
          authority: ['sys:platform_admin', 'sys:tenant_manager'],
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
          authority: ['sys:platform_admin', 'sys:tenant_manager'],
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
          authority: ['sys:platform_admin', 'sys:tenant_manager'],
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
          authority: ['sys:platform_admin', 'sys:tenant_manager'],
        },
        component: () => import('#/views/app/data-analysis/profile/index.vue'),
      },
      {
        path: 'attribution',
        name: 'AttributionAnalysis',
        meta: {
          order: 10,
          icon: 'lucide:target',
          title: $t('menu.dataAnalysis.attribution'),
          authority: ['sys:platform_admin', 'sys:tenant_manager'],
        },
        component: () =>
          import('#/views/app/data-analysis/attribution/index.vue'),
      },
      {
        path: 'distribution',
        name: 'DistributionAnalysis',
        meta: {
          order: 11,
          icon: 'lucide:bar-chart-big',
          title: $t('menu.dataAnalysis.distribution'),
          authority: ['sys:platform_admin', 'sys:tenant_manager'],
        },
        component: () =>
          import('#/views/app/data-analysis/distribution/index.vue'),
      },
      {
        path: 'behavior-sequence',
        name: 'BehaviorSequence',
        meta: {
          order: 12,
          icon: 'lucide:git-commit-horizontal',
          title: $t('menu.dataAnalysis.behaviorSequence'),
          authority: ['sys:platform_admin', 'sys:tenant_manager'],
        },
        component: () =>
          import('#/views/app/data-analysis/behavior-sequence/index.vue'),
      },
      {
        path: 'segmentation',
        name: 'Segmentation',
        meta: {
          order: 13,
          icon: 'lucide:users',
          title: $t('menu.dataAnalysis.segmentation'),
          authority: ['sys:platform_admin', 'sys:tenant_manager'],
        },
        component: () =>
          import('#/views/app/data-analysis/segmentation/index.vue'),
      },
      {
        path: 'click',
        name: 'ClickHeatmap',
        meta: {
          order: 14,
          icon: 'lucide:mouse-pointer-click',
          title: $t('menu.dataAnalysis.click'),
          authority: ['sys:platform_admin', 'sys:tenant_manager'],
        },
        component: () => import('#/views/app/data-analysis/click/index.vue'),
      },
      {
        path: 'lifecycle',
        name: 'Lifecycle',
        meta: {
          order: 15,
          icon: 'lucide:git-fork',
          title: $t('menu.dataAnalysis.lifecycle'),
          authority: ['sys:platform_admin', 'sys:tenant_manager'],
        },
        component: () =>
          import('#/views/app/data-analysis/lifecycle/index.vue'),
      },
      {
        path: 'churn',
        name: 'Churn',
        meta: {
          order: 16,
          icon: 'lucide:user-minus',
          title: $t('menu.dataAnalysis.churn'),
          authority: ['sys:platform_admin', 'sys:tenant_manager'],
        },
        component: () => import('#/views/app/data-analysis/churn/index.vue'),
      },
      {
        path: 'interval',
        name: 'Interval',
        meta: {
          order: 17,
          icon: 'lucide:timer',
          title: $t('menu.dataAnalysis.interval'),
          authority: ['sys:platform_admin', 'sys:tenant_manager'],
        },
        component: () => import('#/views/app/data-analysis/interval/index.vue'),
      },
      {
        path: 'matrix',
        name: 'Matrix',
        meta: {
          order: 18,
          icon: 'lucide:layout-grid',
          title: $t('menu.dataAnalysis.matrix'),
          authority: ['sys:platform_admin', 'sys:tenant_manager'],
        },
        component: () => import('#/views/app/data-analysis/matrix/index.vue'),
      },
      {
        path: 'revenue',
        name: 'Revenue',
        meta: {
          order: 19,
          icon: 'lucide:dollar-sign',
          title: $t('menu.dataAnalysis.revenue'),
          authority: ['sys:platform_admin', 'sys:tenant_manager'],
        },
        component: () => import('#/views/app/data-analysis/revenue/index.vue'),
      },
      {
        path: 'session-analysis',
        name: 'SessionAnalysis',
        meta: {
          order: 20,
          icon: 'lucide:waypoints',
          title: $t('menu.dataAnalysis.sessionAnalysis'),
          authority: ['sys:platform_admin', 'sys:tenant_manager'],
        },
        component: () =>
          import('#/views/app/data-analysis/session-analysis/index.vue'),
      },
      {
        path: 'anomaly',
        name: 'Anomaly',
        meta: {
          order: 21,
          icon: 'lucide:activity',
          title: $t('menu.dataAnalysis.anomaly'),
          authority: ['sys:platform_admin', 'sys:tenant_manager'],
        },
        component: () => import('#/views/app/data-analysis/anomaly/index.vue'),
      },
      {
        path: 'new-vs-old',
        name: 'NewVsOld',
        meta: {
          order: 22,
          icon: 'lucide:users-round',
          title: $t('menu.dataAnalysis.newVsOld'),
          authority: ['sys:platform_admin', 'sys:tenant_manager'],
        },
        component: () =>
          import('#/views/app/data-analysis/new-vs-old/index.vue'),
      },
      {
        path: 'path-sankey',
        name: 'PathSankey',
        meta: {
          order: 23,
          icon: 'lucide:share-2',
          title: $t('menu.dataAnalysis.pathSankey'),
          authority: ['sys:platform_admin', 'sys:tenant_manager'],
        },
        component: () =>
          import('#/views/app/data-analysis/path-sankey/index.vue'),
      },
      {
        path: 'level-analysis',
        name: 'LevelAnalysis',
        meta: {
          order: 24,
          icon: 'lucide:triangle-right',
          title: $t('menu.dataAnalysis.levelAnalysis'),
          authority: ['sys:platform_admin', 'sys:tenant_manager'],
        },
        component: () =>
          import('#/views/app/data-analysis/level-analysis/index.vue'),
      },
      {
        path: 'whale-tier',
        name: 'WhaleTier',
        meta: {
          order: 25,
          icon: 'lucide:gem',
          title: $t('menu.dataAnalysis.whaleTier'),
          authority: ['sys:platform_admin', 'sys:tenant_manager'],
        },
        component: () =>
          import('#/views/app/data-analysis/whale-tier/index.vue'),
      },
      {
        path: 'ltv',
        name: 'LTV',
        meta: {
          order: 26,
          icon: 'lucide:trending-up',
          title: $t('menu.dataAnalysis.ltv'),
          authority: ['sys:platform_admin', 'sys:tenant_manager'],
        },
        component: () => import('#/views/app/data-analysis/ltv/index.vue'),
      },
      {
        path: 'server-retention',
        name: 'ServerRetention',
        meta: {
          order: 27,
          icon: 'lucide:server',
          title: $t('menu.dataAnalysis.serverRetention'),
          authority: ['sys:platform_admin', 'sys:tenant_manager'],
        },
        component: () =>
          import('#/views/app/data-analysis/server-retention/index.vue'),
      },
      {
        path: 'online-stats',
        name: 'OnlineStats',
        meta: {
          order: 28,
          icon: 'lucide:radio',
          title: $t('menu.dataAnalysis.onlineStats'),
          authority: ['sys:platform_admin', 'sys:tenant_manager'],
        },
        component: () =>
          import('#/views/app/data-analysis/online-stats/index.vue'),
      },
      {
        path: 'economy',
        name: 'Economy',
        meta: {
          order: 29,
          icon: 'lucide:coins',
          title: $t('menu.dataAnalysis.economy'),
          authority: ['sys:platform_admin', 'sys:tenant_manager'],
        },
        component: () => import('#/views/app/data-analysis/economy/index.vue'),
      },
    ],
  },
];

export default dataAnalysis;
