<script lang="ts" setup>
import type { VxeGridProps } from '#/adapter/vxe-table';

import { computed, ref } from 'vue';

import { Page, type VbenFormProps } from '@vben/common-ui';

import {
  Card,
  InputNumber,
  message,
  notification,
  Progress,
  Statistic,
} from 'ant-design-vue';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import {
  fetchListAiUsageLogs,
  PaginationQuery,
  useAiUsageSummary,
  useGetTenantQuota,
  useSetTenantQuota,
  serverExportFile,
} from '#/api';
import { $t } from '#/locales';
import { downloadTableFile } from '#/utils/csv';

// 当前搜索条件（导出透传用，与 Grid proxy 查询同源——"导出的就是当前搜索看到的"）
const latestFormValuesRef = ref<Record<string, unknown>>({});

// 服务端全量导出（后端 :export 手动锚点；租户口径与列表接口同语义）。
async function handleServerExport() {
  try {
    await serverExportFile(
      'admin/v1/ai/usage-logs:export',
      new PaginationQuery({ formValues: latestFormValuesRef.value }),
    );
    message.success($t('page.auditExport.serverSuccess'));
  } catch (error: any) {
    // 原始错误必须留在控制台：用户可见的那句翻译不包含服务端的原因
    console.error('[export] server-side export failed', error);
    message.error(error?.message || $t('page.auditExport.serverFailed'));
  }
}


defineOptions({ name: 'AiUsage' });

const summary = useAiUsageSummary();

// 配额进度（0~100，封顶；超限变红由 status 控制）
const quotaPercent = computed(() => {
  const used = summary.data.value?.monthTokens ?? 0;
  const limit = Number(summary.data.value?.quotaLimit ?? 0);
  if (limit <= 0) return 0;
  return Math.min(100, Math.round((used / limit) * 100));
});
const quotaExceeded = computed(
  () =>
    (summary.data.value?.monthTokens ?? 0) >=
    Number(summary.data.value?.quotaLimit ?? 0),
);

// 租户配额查看 / 设置（平台管理员）
const quotaTenantId = ref<number | undefined>();
const quotaLimitInput = ref<number | undefined>();
const quotaInfo = ref<{
  configured?: boolean;
  monthlyTokenLimit?: number | string;
}>();
const { mutateAsync: getTenantQuota, isPending: queryingQuota } =
  useGetTenantQuota();
const { mutateAsync: setTenantQuota, isPending: savingQuota } =
  useSetTenantQuota();

async function handleQueryQuota() {
  if (!quotaTenantId.value) {
    message.warning($t('page.ai.usage.tenantRequired'));
    return;
  }
  try {
    const resp = await getTenantQuota({ tenantId: quotaTenantId.value });
    quotaInfo.value = resp;
    quotaLimitInput.value = Number(resp.monthlyTokenLimit ?? 0);
  } catch {
    notification.error({ message: $t('page.ai.usage.quotaQueryFailed') });
  }
}

async function handleSaveQuota() {
  if (!quotaTenantId.value) {
    message.warning($t('page.ai.usage.tenantRequired'));
    return;
  }
  try {
    await setTenantQuota({
      tenantId: quotaTenantId.value,
      monthlyTokenLimit: quotaLimitInput.value ?? 0,
    } as never);
    notification.success({ message: $t('page.ai.usage.quotaSaveSuccess') });
    await summary.refetch();
  } catch (error: any) {
    notification.error({
      message: $t('page.ai.usage.quotaSaveFailed'),
      description: error?.response?.data?.message || String(error),
    });
  }
}

const exporting = ref(false);

// 导出全部（CSV/Excel 客户端聚合，上限 1 万行）：透传当前搜索条件——导出的即当前搜索看到的
async function handleExportAll(info: { key: string | number }) {
  const format = String(info.key);
  if (format === 'server-xlsx') {
    await handleServerExport();
    return;
  }
  if (format !== 'csv' && format !== 'xlsx') return;
  exporting.value = true;
  try {
    const pageSize = 1000;
    const maxRows = 10000;
    const rows: any[] = [];
    for (let p = 1; rows.length < maxRows; p++) {
      const resp = await fetchListAiUsageLogs(
        new PaginationQuery({
          paging: { page: p, pageSize },
          formValues:
            latestFormValuesRef.value &&
            Object.keys(latestFormValuesRef.value).length > 0
              ? { ...latestFormValuesRef.value }
              : undefined,
        }),
      );
      const items = (resp.items ?? []) as any[];
      rows.push(...items);
      if (items.length < pageSize) break;
    }
    const gridColumns = (gridOptions.columns ?? []) as any[];
    const cols = gridColumns
      .filter((c) => c.field)
      .map((c) => ({ title: c.title as string, key: c.field as string }));
    const sliced = rows.slice(0, maxRows);
    await downloadTableFile(format, `ai-usage-logs-${Date.now()}`, cols, sliced);
    message.success($t('ui.export.success', { count: sliced.length }));
  } catch (error: any) {
    // 原始错误必须留在控制台：翻译文案不含原因
    console.error('[export] client-side export failed', error);
    message.error(error?.message || $t('ui.export.failed'));
  } finally {
    exporting.value = false;
  }
}

const formOptions: VbenFormProps = {
  collapsed: false,
  showCollapseButton: false,
  submitOnEnter: true,
  schema: [
    {
      component: 'Input',
      fieldName: 'modelName__contains',
      label: $t('page.ai.usage.modelName'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
  ],
};

const gridOptions: VxeGridProps = {
  toolbarConfig: {
    custom: true,
    export: true,
    refresh: true,
    zoom: true,
  },
  height: 'auto',
  exportConfig: {},
  pagerConfig: {},
  rowConfig: {
    isHover: true,
  },
  stripe: true,

  proxyConfig: {
    ajax: {
      query: async ({ page }, formValues) => {
        latestFormValuesRef.value =
          formValues && Object.keys(formValues).length > 0 ? { ...formValues } : {};
        return await fetchListAiUsageLogs(
          new PaginationQuery({
            paging: { page: page.currentPage, pageSize: page.pageSize },
            formValues,
          }),
        );
      },
    },
  },

  columns: [
    {
      title: $t('page.ai.usage.modelName'),
      field: 'modelName',
      fixed: 'left',
      minWidth: 180,
    },
    {
      title: $t('page.ai.usage.promptTokens'),
      field: 'promptTokens',
      minWidth: 110,
    },
    {
      title: $t('page.ai.usage.completionTokens'),
      field: 'completionTokens',
      minWidth: 110,
    },
    {
      title: $t('page.ai.usage.totalTokens'),
      field: 'totalTokens',
      minWidth: 110,
    },
    {
      title: $t('page.ai.usage.durationMs'),
      field: 'durationMs',
      minWidth: 100,
    },
    {
      title: $t('page.ai.usage.conversationId'),
      field: 'conversationId',
      minWidth: 110,
    },
    {
      title: $t('ui.table.createdAt'),
      field: 'createdAt',
      formatter: 'formatDateTime',
      minWidth: 140,
    },
  ],
};

const [Grid] = useVbenVxeGrid({ gridOptions, formOptions });
</script>

<template>
  <Page auto-content-height>
    <div class="mb-4 grid grid-cols-2 gap-4 lg:grid-cols-4">
      <Card size="small">
        <Statistic
          :title="$t('page.ai.usage.monthTokens')"
          :value="summary.data.value?.monthTokens ?? 0"
        />
      </Card>
      <Card size="small">
        <Statistic
          :title="$t('page.ai.usage.monthCalls')"
          :value="summary.data.value?.monthCalls ?? 0"
        />
      </Card>
      <!-- 配额进度（本租户配置了月度配额时显示） -->
      <Card v-if="summary.data.value?.quotaConfigured" size="small">
        <div class="flex flex-col gap-1">
          <span class="text-sm opacity-60">
            {{ $t('page.ai.usage.quotaTitle') }}
          </span>
          <Progress
            :percent="quotaPercent"
            :status="quotaExceeded ? 'exception' : 'normal'"
            size="small"
          />
          <span class="text-xs opacity-60">
            {{ summary.data.value?.monthTokens ?? 0 }} /
            {{ summary.data.value?.quotaLimit ?? 0 }}
          </span>
        </div>
      </Card>
    </div>

    <!-- 租户配额管理（仅平台管理员可保存成功；租户用户查询会被强制指向本租户） -->
    <Card size="small" class="mb-4" :title="$t('page.ai.usage.quotaManage')">
      <div class="flex flex-wrap items-center gap-3">
        <InputNumber
          v-model:value="quotaTenantId"
          :min="1"
          :placeholder="$t('page.ai.usage.tenantId')"
          class="w-40"
        />
        <a-button :loading="queryingQuota" @click="handleQueryQuota">
          {{ $t('page.ai.usage.quotaQuery') }}
        </a-button>
        <span v-if="quotaInfo" class="text-sm">
          {{
            quotaInfo.configured
              ? $t('page.ai.usage.quotaConfigured', {
                  limit: String(quotaInfo.monthlyTokenLimit ?? 0),
                })
              : $t('page.ai.usage.quotaUnlimited')
          }}
        </span>
        <div class="flex items-center gap-2">
          <InputNumber
            v-model:value="quotaLimitInput"
            :min="0"
            :placeholder="$t('page.ai.usage.monthlyLimit')"
            class="w-52"
          />
          <a-button
            type="primary"
            :loading="savingQuota"
            @click="handleSaveQuota"
          >
            {{ $t('page.ai.usage.quotaSave') }}
          </a-button>
          <span class="text-xs opacity-60">
            {{ $t('page.ai.usage.quotaHint') }}
          </span>
        </div>
      </div>
    </Card>

    <Grid :table-title="$t('menu.ai.usage')">
      <template #toolbar-tools>
        <a-dropdown class="mr-2">
          <a-button :loading="exporting">{{ $t('ui.button.exportAll') }}</a-button>
          <template #overlay>
            <a-menu @click="handleExportAll">
              <a-menu-item key="csv">{{ $t('ui.button.exportFormatCsv') }}</a-menu-item>
              <a-menu-item key="xlsx">{{ $t('ui.button.exportFormatXlsx') }}</a-menu-item>
              <a-menu-divider />
              <a-menu-item key="server-xlsx">{{ $t('page.auditExport.serverFull') }}</a-menu-item>
            </a-menu>
          </template>
        </a-dropdown>
      </template>
    </Grid>
  </Page>
</template>
