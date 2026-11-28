import { getWx, isMiniProgramHost } from './wx-env';
import { DEVICE_ID_KEY, storageGet, storageSet } from './storage';
import { osDisplayName, trimAndLimit, uuidV4, SDK_VERSION } from './utils';

/** 初始化后不再变化的客户端环境快照。 */
export interface ContextSnapshot {
  deviceId: string;
  deviceIdIsNew: boolean;
  platform: string;
  os: string;
  userAgent: string;
}

/** 加载（或生成并持久化）设备环境。SDK 初始化时调用一次。 */
export function loadContext(configuredPlatform?: string): ContextSnapshot {
  const persisted = storageGet(DEVICE_ID_KEY);
  const deviceIdIsNew = !persisted;
  const deviceId = deviceIdIsNew ? uuidV4() : persisted;
  if (deviceIdIsNew) storageSet(DEVICE_ID_KEY, deviceId);

  const wx = getWx();
  let info: ReturnType<typeof wx.getSystemInfoSync> = {};
  try {
    info = wx.getSystemInfoSync();
  } catch {
    // devtools 之外的极端环境兜底
  }

  const platform =
    configuredPlatform ?? (isMiniProgramHost() ? 'mini_program' : 'mini_game');
  const os = osDisplayName(info.system);
  const model = trimAndLimit(info.model, 64) ?? '';
  const system = trimAndLimit(info.system, 64) ?? '';
  const host = trimAndLimit(info.platform, 32) ?? platform;
  const userAgent = trimAndLimit(
    `uba-wechat/${SDK_VERSION} (${[model, system, host].filter(Boolean).join('; ')})`,
    256,
  ) ?? `uba-wechat/${SDK_VERSION}`;

  return { deviceId, deviceIdIsNew, platform, os, userAgent };
}
