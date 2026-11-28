<script lang="ts" setup>
import type { VbenFormSchema } from '@vben/common-ui';

import { computed, onMounted, ref } from 'vue';

import { AuthenticationLogin, VbenButton, z } from '@vben/common-ui';
import { $t } from '@vben/locales';

import { fetchCapabilities, ssoAuthorize } from '#/api/composables';
import { useAuthStore } from '#/stores';

defineOptions({ name: 'Login' });

const authStore = useAuthStore();

/** 企业单点登录是否可用(仅企业构建且 license 授权 sso 能力时为真)。 */
const ssoEnabled = ref(false);
const ssoJumping = ref(false);

onMounted(async () => {
  try {
    const capabilities = await fetchCapabilities();
    ssoEnabled.value = capabilities.includes('sso');
  } catch {
    ssoEnabled.value = false;
  }
});

async function onSsoLogin() {
  if (ssoJumping.value) return;
  ssoJumping.value = true;
  try {
    const resp = await ssoAuthorize();
    if (resp?.authorize_url) {
      window.location.href = resp.authorize_url;
    }
  } finally {
    ssoJumping.value = false;
  }
}

const formSchema = computed((): VbenFormSchema[] => {
  return [
    {
      component: 'VbenInput',
      componentProps: {
        placeholder: $t('authentication.usernameTip'),
      },
      dependencies: {
        trigger(values, _form) {
          if (values.selectAccount) {
          }
        },
        triggerFields: ['selectAccount'],
      },
      fieldName: 'username',
      label: $t('authentication.username'),
      rules: z.string().min(1, { message: $t('authentication.usernameTip') }),
    },
    {
      component: 'VbenInputPassword',
      componentProps: {
        placeholder: $t('authentication.password'),
      },
      fieldName: 'password',
      label: $t('authentication.password'),
      rules: z.string().min(1, { message: $t('authentication.passwordTip') }),
    },
  ];
});
</script>

<template>
  <AuthenticationLogin
    :form-schema="formSchema"
    :loading="authStore.loginLoading"
    :show-code-login="false"
    :show-forget-password="false"
    :show-qrcode-login="false"
    :show-register="true"
    :show-third-party-login="false"
    @submit="authStore.authLogin"
  />
  <VbenButton
    v-if="ssoEnabled"
    :loading="ssoJumping"
    class="mt-3 w-full"
    variant="outline"
    @click="onSsoLogin"
  >
    {{ $t('authentication.ssoLogin') }}
  </VbenButton>
</template>
