/**
 * 工具函数
 */

/** 生成 RFC4122 v4 UUID。优先用原生 crypto.randomUUID，回退到 Math.random 实现。 */
export function uuid(): string {
  // 浏览器与 Node 16.7+ 均支持 crypto.randomUUID
  const g: any = globalThis as any;
  if (g.crypto && typeof g.crypto.randomUUID === 'function') {
    return g.crypto.randomUUID();
  }
  // 回退实现
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0;
    const v = c === 'x' ? r : (r & 0x3) | 0x8;
    return v.toString(16);
  });
}

/** 生成 RFC3339 时间字符串（UTC），如 2026-06-28T08:30:00.000Z */
export function toRFC3339(d: Date = new Date()): string {
  return d.toISOString();
}

/** 浅合并多个对象，跳过值为 undefined 的键。返回新对象，不修改入参。 */
export function merge<T extends Record<string, any>>(...sources: (T | undefined | null)[]): T {
  const result: Record<string, any> = {};
  for (const src of sources) {
    if (!src) {
      continue;
    }
    for (const key of Object.keys(src)) {
      const v = (src as any)[key];
      if (v !== undefined) {
        result[key] = v;
      }
    }
  }
  return result as T;
}

/**
 * 去除首尾空格并按字符（rune）数限制最大长度。
 * 注意：按 rune 截断而非字节，避免切断 UTF-8 多字节字符（如中文）。
 */
export function trimAndLimit(s: string | undefined | null, max: number): string {
  if (!s) {
    return '';
  }
  const t = s.trim();
  // Array.from 按 code point 拆分，正确处理多字节字符
  const runes = Array.from(t);
  if (runes.length > max) {
    return runes.slice(0, max).join('');
  }
  return t;
}

/** 简单 sleep，用于重试退避 */
export function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
