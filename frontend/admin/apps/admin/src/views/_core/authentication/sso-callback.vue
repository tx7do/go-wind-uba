<script lang="ts" setup>
import { onMounted } from 'vue';
import { useRoute } from 'vue-router';

import { Spin } from 'ant-design-vue';
import { $t } from '@vben/locales';

import { useAuthStore } from '#/stores';

defineOptions({ name: 'SsoCallback' });

const route = useRoute();
const authStore = useAuthStore();

onMounted(() => {
  const code = (route.query.code as string) ?? '';
  const state = (route.query.state as string) ?? '';
  authStore.ssoLogin({ code, state });
});
</script>

<template>
  <div class="flex h-full w-full items-center justify-center">
    <Spin :spinning="true" :tip="$t('authentication.ssoLoggingIn')" size="large" />
  </div>
</template>
