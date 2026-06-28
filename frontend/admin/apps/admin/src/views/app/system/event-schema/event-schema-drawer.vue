<script lang="ts" setup>
import type { ubaservicev1_EventPropertySchema } from '#/generated/api/admin/service/v1';

import { computed, ref } from 'vue';

import { useVbenDrawer } from '@vben/common-ui';

import { notification } from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import {
  eventPropertyTypeList,
  eventSchemaStatusList,
  useCreateEventSchema,
  useUpdateEventSchema,
} from '#/api';
import { $t } from '#/locales';

const { mutateAsync: createEventSchema } = useCreateEventSchema();
const { mutateAsync: updateEventSchema } = useUpdateEventSchema();

const data = ref<any>();

const getTitle = computed(() =>
  data.value?.create
    ? $t('page.eventSchema.button.create')
    : $t('page.eventSchema.button.update'),
);

// 属性 schema 列表（可增删的子表）
const properties = ref<ubaservicev1_EventPropertySchema[]>([]);

const [BaseForm, baseFormApi] = useVbenForm({
  showDefaultActions: false,
  schema: [
    {
      component: 'Input',
      fieldName: 'eventName',
      label: $t('page.analytics.eventName'),
      rules: 'required',
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'displayName',
      label: $t('page.eventSchema.displayName'),
      rules: 'required',
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'category',
      label: $t('page.eventSchema.category'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Textarea',
      fieldName: 'description',
      label: $t('page.eventSchema.description'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Select',
      fieldName: 'status',
      label: $t('page.eventSchema.status'),
      defaultValue: 'ENABLED',
      componentProps: {
        class: 'w-full',
        options: eventSchemaStatusList.map((i) => ({
          label: i.label,
          value: i.value,
        })),
        placeholder: $t('ui.placeholder.select'),
        allowClear: true,
      },
    },
  ],
});

function addProperty() {
  properties.value.push({
    name: '',
    type: 'string',
    required: false,
  });
}

function removeProperty(index: number) {
  properties.value.splice(index, 1);
}

const [Drawer, drawerApi] = useVbenDrawer({
  onConfirm: async () => {
    const validate = await baseFormApi.validate();
    if (!validate.valid) {
      return;
    }

    setLoading(true);
    const values = await baseFormApi.getValues();

    // 组装 EventSchema 数据对象
    const payload = {
      ...values,
      properties: properties.value,
    } as any;

    try {
      await (data.value?.create
        ? createEventSchema(payload)
        : updateEventSchema({
            id: data.value.row.id,
            values: payload,
          }));
      notification.success({
        message: data.value?.create
          ? $t('ui.notification.create_success')
          : $t('ui.notification.update_success'),
      });
    } catch {
      notification.error({
        message: data.value?.create
          ? $t('ui.notification.create_failed')
          : $t('ui.notification.update_failed'),
      });
    } finally {
      drawerApi.close();
      setLoading(false);
    }
  },

  onOpenChange(isOpen) {
    if (isOpen) {
      data.value = drawerApi.getData<Record<string, any>>();
      baseFormApi.setValues(data.value?.row);
      properties.value =
        (data.value?.row?.properties as ubaservicev1_EventPropertySchema[]) ??
        [];
      setLoading(false);
    }
  },
});

function setLoading(loading: boolean) {
  drawerApi.setState({ loading });
}
</script>

<template>
  <Drawer :title="getTitle">
    <BaseForm class="mx-4" />

    <!-- 属性 schema 编辑子表 -->
    <div class="mx-4 mt-4">
      <div class="mb-2 flex items-center justify-between">
        <span class="font-medium">{{
          $t('page.analytics.schemaProperty')
        }}</span>
        <a-button type="dashed" size="small" @click="addProperty">
          + {{ $t('page.analytics.addProperty') }}
        </a-button>
      </div>
      <a-table
        :data-source="properties"
        :pagination="false"
        row-key="name"
        size="small"
      >
        <a-table-column
          :title="$t('page.analytics.schemaProperty')"
          data-index="name"
          :width="160"
        >
          <template #default="{ record }">
            <a-input
              v-model:value="record.name"
              placeholder="name"
              size="small"
            />
          </template>
        </a-table-column>
        <a-table-column
          :title="$t('page.analytics.schemaPropertyType')"
          :width="140"
        >
          <template #default="{ record }">
            <a-select v-model:value="record.type" size="small" class="w-full">
              <a-select-option
                v-for="t in eventPropertyTypeList"
                :key="t"
                :value="t"
              >
                {{ t }}
              </a-select-option>
            </a-select>
          </template>
        </a-table-column>
        <a-table-column
          :title="$t('page.analytics.schemaRequired')"
          :width="80"
        >
          <template #default="{ record }">
            <a-switch v-model:checked="record.required" size="small" />
          </template>
        </a-table-column>
        <a-table-column :width="60">
          <template #default="{ index }">
            <a-button
              v-if="properties.length > 0"
              type="link"
              danger
              size="small"
              @click="removeProperty(index)"
            >
              ×
            </a-button>
          </template>
        </a-table-column>
      </a-table>
    </div>
  </Drawer>
</template>
