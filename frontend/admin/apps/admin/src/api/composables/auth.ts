import type {
  authenticationservicev1_LoginRequest,
  authenticationservicev1_LoginResponse,
} from '#/generated/api/admin/service/v1';

import {
  useMutation,
  type UseMutationOptions,
} from '@tanstack/vue-query';

import { apiClient } from '#/api/client';
import { queryClient } from '#/plugins/vue-query';

// 供非 Vue 上下文使用的纯函数
export async function login(request: authenticationservicev1_LoginRequest) {
  return apiClient.authenticationService.Login(request);
}

export async function logout() {
  return apiClient.authenticationService.Logout({});
}

export async function refreshToken(refreshToken: string) {
  return apiClient.authenticationService.RefreshToken({
    grant_type: 'refresh_token',
    refresh_token: refreshToken ?? '',
  });
}

// ------------------------------
// 企业单点登录（开源构建恒不可用，按 capabilities 显隐）
// ------------------------------

/** 查询平台已授权的企业能力清单(公开接口)。 */
export async function fetchCapabilities(): Promise<string[]> {
  const resp = await apiClient.authenticationService.Capabilities({});
  return resp?.capabilities ?? [];
}

/** 生成 IdP 授权地址与 state;前端拿到 authorize_url 后整页跳转。 */
export async function ssoAuthorize() {
  return apiClient.authenticationService.SsoAuthorize({});
}

/** 授权码换平台令牌;返回与密码登录同构的令牌结构。 */
export async function ssoCallback(code: string, state: string) {
  return apiClient.authenticationService.SsoCallback({
    code,
    provider: '',
    state,
  });
}

// ------------------------------
// 登录（Mutation）
// ------------------------------
export function useLogin(
  options?: UseMutationOptions<
    authenticationservicev1_LoginResponse,
    Error,
    authenticationservicev1_LoginRequest
  >,
) {
  return useMutation({
    mutationFn: (req) => login(req),
    ...options,
  });
}

// ------------------------------
// 登录（预构建 Mutation，供 Store / 非 Vue 上下文使用）
// ------------------------------
export const loginMutation = queryClient.getMutationCache().build(queryClient, {
  mutationKey: ['login'],
  mutationFn: login,
  retry: 0,
});

// ------------------------------
// 登出（Mutation）
// ------------------------------
export function useLogout(options?: UseMutationOptions<object, Error, object>) {
  return useMutation({
    mutationFn: () => logout(),
    ...options,
  });
}

// ------------------------------
// 登出（预构建 Mutation，供 Store / 非 Vue 上下文使用）
// ------------------------------
export const logoutMutation = queryClient
  .getMutationCache()
  .build(queryClient, {
    mutationKey: ['logout'],
    mutationFn: logout,
    retry: 0,
  });

// ------------------------------
// 刷新 Token（Mutation）
// ------------------------------
export function useRefreshToken(
  options?: UseMutationOptions<
    authenticationservicev1_LoginResponse,
    Error,
    authenticationservicev1_LoginRequest
  >,
) {
  return useMutation({
    mutationFn: (req) => refreshToken(req.refresh_token ?? ''),
    ...options,
  });
}

// ------------------------------
// 刷新 Token（预构建 Mutation，供 Store / 非 Vue 上下文使用）
// ------------------------------
export const refreshTokenMutation = queryClient
  .getMutationCache()
  .build(queryClient, {
    mutationKey: ['refreshToken'],
    mutationFn: refreshToken,
    retry: 0,
  });
