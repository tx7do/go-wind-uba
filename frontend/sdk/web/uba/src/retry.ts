/**
 * 重试与降级
 *
 * - 网络失败/超时：指数退避重试
 * - 鉴权失败（401）/客户端错误（4xx，除 429）：不重试
 * - 超过重试上限：丢弃事件（避免内存无限增长），记录 warn
 */

import { sleep } from './utils';
import type { PostReportResponse, KratosError } from './types';

/** 不应重试的 HTTP 状态码：4xx 客户端错误（429 除外） */
function isNoRetryStatus(status: number): boolean {
  return status >= 400 && status < 500 && status !== 429;
}

/** fetch 请求结果 */
export interface FetchResult {
  ok: boolean;
  status: number;
  /** 成功时的响应体；失败时为 undefined */
  response?: PostReportResponse;
  /** 失败时的 Kratos 错误信封；成功或非标准错误时为 undefined */
  error?: KratosError;
  /** 抛出的异常信息（网络错误、超时、JSON 解析失败等） */
  exception?: string;
}

export interface RetryConfig {
  /** 最大重试次数，默认 3 */
  maxRetries: number;
  /** 单次请求超时（毫秒） */
  timeout: number;
  /** 重试基础退避（毫秒），实际退避 = base * 2^attempt */
  baseDelay: number;
  /** 日志函数 */
  log: (level: 'warn' | 'error', msg: string) => void;
}

/**
 * 发送单次请求（带超时）。
 * 使用 AbortController 实现超时。
 */
async function sendOnce(
  url: string,
  body: string,
  timeout: number,
): Promise<FetchResult> {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeout);
  try {
    const resp = await fetch(url, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      body,
      signal: controller.signal,
      // 避免携带 cookie 干扰鉴权
      credentials: 'omit',
      keepalive: false,
    });
    const text = await resp.text();
    let parsed: any;
    try {
      parsed = text ? JSON.parse(text) : {};
    } catch {
      return { ok: false, status: resp.status, exception: 'invalid JSON response' };
    }
    if (resp.ok) {
      return { ok: true, status: resp.status, response: parsed as PostReportResponse };
    }
    return { ok: false, status: resp.status, error: parsed as KratosError };
  } catch (e: any) {
    const aborted = e && e.name === 'AbortError';
    return {
      ok: false,
      status: 0,
      exception: aborted ? `request timeout (${timeout}ms)` : String(e?.message || e),
    };
  } finally {
    clearTimeout(timer);
  }
}

/**
 * 带重试的发送。返回最终结果（成功或耗尽重试后的失败）。
 *
 * @param url 上报地址
 * @param body 请求体 JSON 字符串
 * @param cfg 重试配置
 * @returns 最终 FetchResult
 */
export async function sendWithRetry(url: string, body: string, cfg: RetryConfig): Promise<FetchResult> {
  let lastResult: FetchResult;
  for (let attempt = 0; attempt <= cfg.maxRetries; attempt++) {
    lastResult = await sendOnce(url, body, cfg.timeout);

    // 成功（HTTP 2xx）即返回
    if (lastResult.ok) {
      return lastResult;
    }

    // 鉴权失败 / 客户端错误：不重试，直接返回
    if (isNoRetryStatus(lastResult.status)) {
      return lastResult;
    }

    // 还有重试机会：退避后重试
    if (attempt < cfg.maxRetries) {
      const delay = cfg.baseDelay * Math.pow(2, attempt);
      cfg.log('warn', `upload failed (attempt ${attempt + 1}), retrying in ${delay}ms: ${resultSummary(lastResult)}`);
      await sleep(delay);
    }
  }
  // 重试耗尽
  cfg.log('error', `upload failed after ${cfg.maxRetries + 1} attempts: ${resultSummary(lastResult!)}`);
  return lastResult!;
}

function resultSummary(r: FetchResult): string {
  if (r.exception) {
    return r.exception;
  }
  if (r.error) {
    return `status=${r.status} reason=${r.error.reason} msg=${r.error.message}`;
  }
  return `status=${r.status}`;
}

/**
 * 使用 navigator.sendBeacon 兜底发送（页面卸载场景）。
 * sendBeacon 不支持读响应、不支持自定义超时，仅作 best-effort 投递。
 */
export function sendBeacon(url: string, body: string): boolean {
  if (typeof navigator === 'undefined' || typeof navigator.sendBeacon !== 'function') {
    return false;
  }
  const blob = new Blob([body], { type: 'application/json' });
  return navigator.sendBeacon(url, blob);
}
