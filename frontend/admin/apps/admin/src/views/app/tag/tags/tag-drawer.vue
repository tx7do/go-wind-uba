<script lang="ts" setup>
import { computed, ref } from 'vue';

import { useVbenDrawer } from '@vben/common-ui';
import { $t } from '@vben/locales';

import { notification } from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import {
  tagCategoryDict,
  tagTypeDict,
  useTagDefinitionListStore,
} from '#/stores';

const tagDefinitionListStore = useTagDefinitionListStore();

const data = ref();

const getTitle = computed(() =>
  data.value?.create
    ? $t('page.tagDefinition.button.create')
    : $t('page.tagDefinition.button.update'),
);

// const isCreate = computed(() => data.value?.create);

const [BaseForm, baseFormApi] = useVbenForm({
  showDefaultActions: false,
  schema: [
    {
      component: 'Input',
      fieldName: 'name',
      label: $t('page.tagDefinition.name'),
      rules: 'required',
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Input',
      fieldName: 'code',
      label: $t('page.tagDefinition.code'),
      rules: 'required',
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Textarea',
      fieldName: 'description',
      label: $t('page.tagDefinition.description'),
      rules: 'required',
      componentProps: {
        placeholder: $t('ui.placeholder.input'),
        allowClear: true,
      },
    },
    {
      component: 'Select',
      fieldName: 'category',
      label: $t('page.tagDefinition.category'),
      rules: 'selectRequired',
      componentProps: {
        options: tagCategoryDict(),
        class: 'w-full',
        placeholder: $t('ui.placeholder.select'),
        filterOption: (input: string, option: any) =>
          option.label.toLowerCase().includes(input.toLowerCase()),
        allowClear: true,
        showSearch: true,
      },
    },
    {
      component: 'Select',
      fieldName: 'tagType',
      label: $t('page.tagDefinition.tagType'),
      rules: 'selectRequired',
      componentProps: {
        options: tagTypeDict(),
        class: 'w-full',
        placeholder: $t('ui.placeholder.select'),
        filterOption: (input: string, option: any) =>
          option.label.toLowerCase().includes(input.toLowerCase()),
        allowClear: true,
        showSearch: true,
      },
    },
    {
      component: 'Switch',
      fieldName: 'isSystem',
      label: $t('page.tagDefinition.isSystem'),
      defaultValue: false,
    },
    {
      component: 'Switch',
      fieldName: 'isDynamic',
      label: $t('page.tagDefinition.isDynamic'),
      defaultValue: false,
    },
    {
      component: 'InputNumber',
      fieldName: 'refreshIntervalSeconds',
      label: $t('page.tagDefinition.refreshIntervalSeconds'),
      rules: 'required',
      componentProps: {
        class: 'w-full',
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
        ? tagDefinitionListStore.createTagDefinition(values)
        : tagDefinitionListStore.updateTagDefinition(
            data.value.row.id,
            values,
          ));

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
