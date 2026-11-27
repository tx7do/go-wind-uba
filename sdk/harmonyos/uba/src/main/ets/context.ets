// context.ets —— 设备 ID 持久化（@ohos.data.preferences，键名 __uba_*__ 对齐各端）。
import preferences from '@ohos.data.preferences'
import { uuidV4 } from './utils'

export const DEVICE_ID_KEY = '__uba_device_id__'

/** 加载（或生成并持久化）设备 ID。应用启动时调用一次。 */
export async function loadDeviceId(context: Context): Promise<string> {
  const store = await preferences.getPreferences(context, 'uba_store')
  const existing = await store.get(DEVICE_ID_KEY, '')
  if (typeof existing === 'string' && existing !== '') {
    return existing
  }
  const deviceId = uuidV4()
  await store.put(DEVICE_ID_KEY, deviceId)
  await store.flush()
  return deviceId
}
