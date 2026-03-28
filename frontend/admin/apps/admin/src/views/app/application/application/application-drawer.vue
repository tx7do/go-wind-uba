<script lang="ts" setup>
import { computed, ref } from 'vue';

import { useVbenDrawer } from '@vben/common-ui';
import { $t } from '@vben/locales';

import { notification } from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import {
  appPlatformDict,
  appTypeDict,
  statusList,
  useApplicationListStore,
} from '#/stores';

const applicationStore = useApplicationListStore();

const data = ref();

const getTitle = computed(() =>
  data.value?.create
    ? $t('page.application.button.create')
    : $t('page.application.button.update'),
);

// const isCreate = computed(() => data.value?.create);

const [BaseForm, baseFormApi] = useVbenForm({
  showDefaultActions: false,
  schema: [
    {
      component: 'Input',
      fieldName: 'name',
      label: $t('page.application.name'),
      rules: 'required',
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'appId',
      label: $t('page.application.appId'),
      rules: 'required',
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'appKey',
      label: $t('page.application.appKey'),
      rules: 'required',
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'appSecret',
      label: $t('page.application.appSecret'),
      rules: 'required',
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Select',
      fieldName: 'type',
      label: $t('page.application.type'),
      rules: 'selectRequired',
      componentProps: {
        options: appTypeDict(),
        placeholder: $t('ui.placeholder.select'),
        class: 'w-full',
        allowClear: true,
        showSearch: true,
        filterOption: (input: string, option: any) =>
          option.label.toLowerCase().includes(input.toLowerCase()),
      },
    },
    {
      component: 'Select',
      fieldName: 'status',
      label: $t('ui.table.status'),
      rules: 'selectRequired',
      defaultValue: 'ON',
      componentProps: {
        options: statusList,
        placeholder: $t('ui.placeholder.select'),
        class: 'w-full',
        allowClear: true,
        showSearch: true,
        filterOption: (input: string, option: any) =>
          option.label.toLowerCase().includes(input.toLowerCase()),
      },
    },
    {
      component: 'Select',
      fieldName: 'platforms',
      label: $t('page.application.platforms'),
      rules: 'selectRequired',
      componentProps: {
        class: 'w-full',
        options: appPlatformDict(),
        placeholder: $t('ui.placeholder.select'),
        showSearch: true,
        allowClear: true,
        mode: 'multiple',
        filterOption: (input: string, option: any) =>
          option.label.toLowerCase().includes(input.toLowerCase()),
      },
    },
    {
      component: 'Input',
      fieldName: 'remark',
      label: $t('page.application.remark'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'webhookUrl',
      label: $t('page.application.webhookUrl'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'webhookSecret',
      label: $t('page.application.webhookSecret'),
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
  ],
});

const [Drawer, drawerApi] = useVbenDrawer({
  onCancel() {
    drawerApi.close();
  },

  async onConfirm() {
    console.log('onConfirm');

    // 校验输入的数据
    const validate = await baseFormApi.validate();
    if (!validate.valid) {
      return;
    }

    setLoading(true);

    // 获取表单数据
    const values = await baseFormApi.getValues();

    console.log(getTitle.value, values);

    try {
      await (data.value?.create
        ? applicationStore.createApplication(values)
        : applicationStore.updateApplication(data.value.row.id, values));

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
      // 获取传入的数据
      data.value = drawerApi.getData<Record<string, any>>();

      // 为表单赋值
      baseFormApi.setValues(data.value?.row);

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
  </Drawer>
</template>
