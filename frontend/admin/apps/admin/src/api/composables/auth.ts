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
