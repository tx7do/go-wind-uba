/** SDK 版本号，与 package.json 保持一致，同时用于 User-Agent。 */
export const SDK_VERSION = '1.0.0';

/** uuid v4（Math.random 实现：小程序/小游戏无 crypto.getRandomValues）。 */
export function uuidV4(): string {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0;
    const v = c === 'x' ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  });
}

/** UTC 时间 → RFC3339 毫秒精度字符串，与其他端 SDK 精度一致。 */
export function toRFC3339Ms(date: Date): string {
  const pad = (n: number, w = 2): string => String(n).padStart(w, '0');
  return (
    `${pad(date.getUTCFullYear(), 4)}-${pad(date.getUTCMonth() + 1)}-${pad(date.getUTCDate())}` +
    `T${pad(date.getUTCHours())}:${pad(date.getUTCMinutes())}:${pad(date.getUTCSeconds())}` +
    `.${pad(date.getUTCMilliseconds(), 3)}Z`
  );
}

/** super 属性在前、调用属性在后（后者覆盖前者）。 */
export function mergeProperties(
  base?: Record<string, string>,
  extra?: Record<string, string>,
): Record<string, string> {
  const merged: Record<string, string> = {};
  if (base) Object.assign(merged, base);
  if (extra) Object.assign(merged, extra);
  return merged;
}

/** 去除首尾空白并按字符截断到 limit（对齐 Web SDK trimAndLimit 语义）。 */
export function trimAndLimit(value?: string | null, limit = 256): string | undefined {
  if (value == null) return undefined;
  const v = value.trim();
  if (!v) return undefined;
  return v.length <= limit ? v : v.slice(0, limit);
}

/** FNV-1a 32 位哈希：按 sessionId 确定性采样（与 Web/Flutter SDK 算法一致）。 */
export function fnv1a32(s: string): number {
  let hash = 0x811c9dc5;
  for (let i = 0; i < s.length; i++) {
    hash ^= s.charCodeAt(i) & 0xff;
    hash = Math.imul(hash, 0x01000193) >>> 0;
    hash ^= (s.charCodeAt(i) >> 8) & 0xff;
    hash = Math.imul(hash, 0x01000193) >>> 0;
  }
  return hash >>> 0;
}

/** 会话是否落在采样区间：hash(sessionId) % 1000 / 1000 < sampleRate。 */
export function sessionIdInSample(sessionId: string, sampleRate: number): boolean {
  if (sampleRate >= 1) return true;
  if (sampleRate <= 0) return false;
  return (fnv1a32(sessionId) % 1000) / 1000 < sampleRate;
}

/** system 字符串（如 "iOS 16.6"）→ 展示用 os 字段。 */
export function osDisplayName(system?: string): string {
  if (!system) return '';
  const name = system.split(/\s+/)[0] ?? '';
  if (/ios/i.test(name)) return 'iOS';
  if (/android/i.test(name)) return 'Android';
  if (/windows/i.test(name)) return 'Windows';
  if (/mac/i.test(name)) return 'macOS';
  return trimAndLimit(name, 32) ?? '';
}
