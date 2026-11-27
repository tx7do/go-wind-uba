import { getWx } from './wx-env';

/** 设备 ID 持久化键（对齐 Web localStorage / Unity PlayerPrefs / Flutter 约定）。 */
export const DEVICE_ID_KEY = '__uba_device_id__';

/** 离线队列持久化键（对齐 Web localStorage）。 */
export const OFFLINE_QUEUE_KEY = '__uba_offline_queue__';

export function storageGet(key: string): string | undefined {
  try {
    const v = getWx().getStorageSync(key);
    return typeof v === 'string' && v !== '' ? v : undefined;
  } catch {
    return undefined;
  }
}

export function storageSet(key: string, value: string): void {
  try {
    getWx().setStorageSync(key, value);
  } catch {
    // 存储满等异常：静默丢弃，不影响主流程
  }
}

export function storageRemove(key: string): void {
  try {
    getWx().removeStorageSync(key);
  } catch {
    // 同上
  }
}
