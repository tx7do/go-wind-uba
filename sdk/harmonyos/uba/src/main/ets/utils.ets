// utils.ets —— uuid / RFC3339 / FNV 采样哈希（与各端 SDK 算法一致）。

export function uuidV4(): string {
  // 随机 v4：HarmonyOS 无 Web Crypto 全量 API，用 Math.random（uuid 仅作去重键）
  let hex = '0123456789abcdef'
  let out = ''
  for (let i = 0; i < 36; i++) {
    if (i === 8 || i === 13 || i === 18 || i === 23) {
      out += '-'
    } else if (i === 14) {
      out += '4'
    } else if (i === 19) {
      const r = Math.floor(Math.random() * 16)
      out += hex.substring(r & 0x3 | 0x8, (r & 0x3 | 0x8) + 1)
    } else {
      const r = Math.floor(Math.random() * 16)
      out += hex.substring(r, r + 1)
    }
  }
  return out
}

function pad(n: number, width: number): string {
  let s = n.toString()
  while (s.length < width) {
    s = '0' + s
  }
  return s
}

/** UTC 毫秒 → RFC3339 毫秒精度（与其他端 SDK 精度一致）。 */
export function toRFC3339Ms(epochMs: number): string {
  const d = new Date(epochMs)
  return `${pad(d.getUTCFullYear(), 4)}-${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())}` +
    `T${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())}:${pad(d.getUTCSeconds())}` +
    `.${pad(d.getUTCMilliseconds(), 3)}Z`
}

export function nowMs(): number {
  return new Date().getTime()
}

/** FNV-1a 32 位哈希：按 sessionId 确定性采样（与各端 SDK 算法一致）。 */
export function fnv1a32(s: string): number {
  let hash = 0x811c9dc5
  for (let i = 0; i < s.length; i++) {
    hash = hash ^ (s.charCodeAt(i) & 0xff)
    hash = Math.imul(hash, 0x01000193) >>> 0
  }
  return hash >>> 0
}

/** 会话是否落在采样区间：hash(sessionId) % 1000 / 1000 < sampleRate。 */
export function sessionIdInSample(sessionId: string, sampleRate: number): boolean {
  if (sampleRate >= 1) {
    return true
  }
  if (sampleRate <= 0) {
    return false
  }
  return (fnv1a32(sessionId) % 1000) / 1000 < sampleRate
}

/** 重试退避：第 attempt 次（从 0 计）= base * 2^attempt。 */
export function backoffDelayMs(baseMs: number, attempt: number): number {
  const capped = attempt > 30 ? 30 : attempt
  return baseMs * Math.pow(2, capped)
}

/** 键值 Map → 普通对象（JSON 序列化用）。 */
export function mapToObject(m: Map<string, string> | null): Record<string, string> | null {
  if (m === null || m.size === 0) {
    return null
  }
  const out: Record<string, string> = {}
  m.forEach((value: string, key: string) => {
    out[key] = value
  })
  return out
}

/** 数值 Map → 普通对象（JSON 序列化用）。 */
export function numberMapToObject(m: Map<string, number> | null): Record<string, number> | null {
  if (m === null || m.size === 0) {
    return null
  }
  const out: Record<string, number> = {}
  m.forEach((value: number, key: string) => {
    out[key] = value
  })
  return out
}
