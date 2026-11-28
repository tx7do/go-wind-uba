import type { WxApi, WxSystemInfo } from '../src/wx-env';

export interface CapturedRequest {
  url: string;
  body: Record<string, unknown>;
}

export interface MockResponse {
  statusCode?: number;
  data?: unknown;
  fail?: boolean;
}

export type Responder = (n: number, url: string) => MockResponse;

export interface WxMock {
  requests: CapturedRequest[];
  storage: Map<string, string>;
  /** 编程式响应：fail=true 走网络失败；statusCode 非 2xx 走 HTTP 失败。 */
  respondWith(fn: Responder): void;
  reset(): void;
  systemInfo(info: WxSystemInfo): void;
  requestCount(): number;
  lastBody(): Record<string, unknown>;
  eventCount(): number;
}

const defaultResponse: MockResponse = {
  statusCode: 200,
  data: { success: true, totalCount: 1, successCount: 1, failedCount: 0 },
};

/** 注入全局 wx 假实现（Map 存储 + 请求捕获）。测试结束自行清理 globalThis.wx。 */
export function installWxMock(): WxMock {
  const requests: CapturedRequest[] = [];
  const storage = new Map<string, string>();
  let responder: Responder = () => defaultResponse;
  let systemInfo: WxSystemInfo = { platform: 'ios', system: 'iOS 16.6', model: 'iPhone 15 Pro', SDKVersion: '3.4.10' };
  let seq = 0;

  const wx: WxApi = {
    request(options) {
      const n = ++seq;
      const bodyStr = typeof options.data === 'string' ? options.data : JSON.stringify(options.data ?? {});
      let parsed: Record<string, unknown> = {};
      try {
        parsed = JSON.parse(bodyStr) as Record<string, unknown>;
      } catch {
        parsed = { __raw: bodyStr };
      }
      requests.push({ url: options.url, body: parsed });
      const res = responder(n, options.url);
      if (res && res.fail) {
        options.fail?.({ errMsg: 'request:fail mock network error' });
        return;
      }
      options.success?.({
        statusCode: res?.statusCode ?? 200,
        data: res?.data ?? defaultResponse.data,
      });
    },
    getStorageSync: (key) => storage.get(key) ?? '',
    setStorageSync: (key, value) => {
      storage.set(key, typeof value === 'string' ? value : JSON.stringify(value));
    },
    removeStorageSync: (key) => {
      storage.delete(key);
    },
    getSystemInfoSync: () => systemInfo,
    onShow: () => {},
    onHide: () => {},
  };

  (globalThis as unknown as { wx?: WxApi }).wx = wx;

  return {
    requests,
    storage,
    respondWith(fn) {
      responder = fn;
    },
    reset() {
      requests.length = 0;
      storage.clear();
      seq = 0;
      responder = () => defaultResponse;
    },
    systemInfo(info) {
      systemInfo = info;
    },
    requestCount: () => requests.length,
    lastBody: () => requests[requests.length - 1].body,
    eventCount: () =>
      requests.reduce((n, r) => n + ((r.body['events'] as unknown[] | undefined)?.length ?? 0), 0),
  };
}

/** 清理全局 wx / Page 注入。 */
export function cleanupGlobals(): void {
  const g = globalThis as unknown as { wx?: unknown; Page?: unknown };
  delete g.wx;
  delete g.Page;
}

/** 模拟小程序宿主（有 Page 全局）。 */
export function setMiniProgramHost(): void {
  (globalThis as unknown as { Page?: unknown }).Page = function Page() {};
}

export function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
