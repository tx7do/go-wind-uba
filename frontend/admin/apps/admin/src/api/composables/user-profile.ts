import type {
  identityservicev1_BindContactRequest,
  identityservicev1_ChangePasswordRequest,
  identityservicev1_UpdateUserRequest,
  identityservicev1_UploadAvatarRequest,
  identityservicev1_UploadAvatarResponse,
  identityservicev1_User,
  identityservicev1_VerifyContactRequest,
} from '#/generated/api/admin/service/v1';

import {
  useMutation,
  type UseMutationOptions,
  useQuery,
  type UseQueryOptions,
} from '@tanstack/vue-query';

import { apiClient } from '#/api/client';
import { queryClient } from '#/plugins/vue-query';
import { makeUpdateMask } from '#/transport/rest';

// 供非 Vue 上下文使用的纯函数
export async function getMe(): Promise<identityservicev1_User | null> {
  return apiClient.userProfileService.GetUser({});
}

export async function updateMyUserInfo(
  request: identityservicev1_UpdateUserRequest,
) {
  return apiClient.userProfileService.UpdateUser(request);
}

export async function changeMyPassword(
  request: identityservicev1_ChangePasswordRequest,
) {
  return apiClient.userProfileService.ChangePassword(request);
}

export async function uploadMyAvatar(
  request: identityservicev1_UploadAvatarRequest,
) {
  return apiClient.userProfileService.UploadAvatar(request);
}

export async function deleteMyAvatar() {
  return apiClient.userProfileService.DeleteAvatar({});
}

export async function bindMyContact(
  request: identityservicev1_BindContactRequest,
) {
  return apiClient.userProfileService.BindContact(request);
}

export async function verifyMyContact(
  request: identityservicev1_VerifyContactRequest,
) {
  return apiClient.userProfileService.VerifyContact(request);
}

export function useGetUserProfile(
  options?: UseQueryOptions<identityservicev1_User | null, Error>,
) {
  return useQuery({
    queryKey: ['getMe'],
    queryFn: () => getMe(),
    ...options,
  });
}

// ==============================================
// 获取用户资料 【给 Store / 外部调用】不用 Hook 的方式
// ==============================================
export async function fetchUserProfile() {
  return queryClient.fetchQuery({
    queryKey: ['userProfile'],
    queryFn: () => getMe(),
    staleTime: 0,
    retry: 0,
  });
}

export function useUpdateUserProfile(
  options?: UseMutationOptions<
    object,
    Error,
    { id: number; values: Record<string, any> }
  >,
) {
  return useMutation({
    mutationFn: ({ id, values }: { id: number; values: Record<string, any> }) =>
      updateMyUserInfo({
        id,
        data: { ...values } as any,
        updateMask: makeUpdateMask(Object.keys(values ?? {})),
      }),
    ...options,
  });
}

export function useChangePassword(
  options?: UseMutationOptions<
    object,
    Error,
    identityservicev1_ChangePasswordRequest
  >,
) {
  return useMutation({
    mutationFn: (data) => changeMyPassword(data),
    ...options,
  });
}

export function useUploadAvatar(
  options?: UseMutationOptions<
    identityservicev1_UploadAvatarResponse,
    Error,
    identityservicev1_UploadAvatarRequest
  >,
) {
  return useMutation({
    mutationFn: (data) => uploadMyAvatar(data),
    ...options,
  });
}

export function useDeleteAvatar(
  options?: UseMutationOptions<object, Error, void>,
) {
  return useMutation({
    mutationFn: () => deleteMyAvatar(),
    ...options,
  });
}

export function useBindContact(
  options?: UseMutationOptions<
    object,
    Error,
    identityservicev1_BindContactRequest
  >,
) {
  return useMutation({
    mutationFn: (data) => bindMyContact(data),
    ...options,
  });
}

export function useVerifyContact(
  options?: UseMutationOptions<
    object,
    Error,
    identityservicev1_VerifyContactRequest
  >,
) {
  return useMutation({
    mutationFn: (data) => verifyMyContact(data),
    ...options,
  });
}

// ==============================
// uba 便捷方法（供组件直接调用，保持原有调用习惯）
// ==============================

function omit<T extends Record<string, any>, K extends string>(
  obj: null | T | undefined,
  keys: K | K[],
): Omit<T, K> {
  if (obj === null || typeof obj !== 'object') return obj as any;
  const result = { ...obj } as Record<string, any>;
  const keysArr = Array.isArray(keys) ? keys : [keys];
  for (const key of keysArr) {
    if (Object.prototype.hasOwnProperty.call(result, key)) {
      delete result[key];
    }
  }
  return result as Omit<T, K>;
}

/** 更新当前用户（自动剥离 password 字段） */
export async function updateUser(values: Record<string, any> = {}) {
  const password = values.password ?? null;
  const cleaned = omit(values, 'password');
  const updateMask = makeUpdateMask(Object.keys(cleaned ?? []));
  return apiClient.userProfileService.UpdateUser({
    data: { ...cleaned } as any,
    password,
    updateMask,
  } as any);
}

/** 修改用户密码 */
export async function changePassword(oldPassword: string, newPassword: string) {
  return apiClient.userProfileService.ChangePassword({
    oldPassword,
    newPassword,
  });
}

/** 上传头像（Base64） */
export async function uploadAvatarBase64(imageBase64: string) {
  return apiClient.userProfileService.UploadAvatar({ imageBase64 });
}

/** 上传头像（图片URL） */
export async function uploadAvatarUrl(imageUrl: string) {
  return apiClient.userProfileService.UploadAvatar({ imageUrl });
}

/** 删除头像 */
export async function deleteAvatar() {
  return apiClient.userProfileService.DeleteAvatar({});
}

/** 绑定手机号 */
export async function bindPhone(phone: string, code: string) {
  return apiClient.userProfileService.BindContact({
    phone: { phone, code },
  });
}

/** 绑定邮箱 */
export async function bindEmail(email: string, verificationCode: string) {
  return apiClient.userProfileService.BindContact({
    email: { email, verificationCode },
  });
}

/** 验证手机号 */
export async function verifyPhone(
  phone: string,
  code: string,
  verificationId?: string,
) {
  return apiClient.userProfileService.VerifyContact({
    phone: { phone, code },
    verificationId,
  });
}

/** 验证邮箱 */
export async function verifyEmail(
  email: string,
  code: string,
  verificationId?: string,
) {
  return apiClient.userProfileService.VerifyContact({
    email: { email, code },
    verificationId,
  });
}
