/**
 * 微信小程序/小游戏 API 的最小类型声明与环境探测。
 *
 * 刻意不依赖 miniprogram-api-typings：本 SDK 只用一小撮 API，
 * 手写声明保持零额外依赖（对齐 Web SDK 纯 tsc 构建的约定）。
 */

export interface WxRequestOptions {
  url: string;
  method?: string;
  data?: unknown;
  header?: Record<string, string>;
  timeout?: number;
  success?: (res: WxRequestResult) => void;
  fail?: (err: { errMsg: string }) => void;
}

export interface WxRequestResult {
  statusCode: number;
  data: unknown;
  errMsg?: string;
}

export interface WxSystemInfo {
  platform?: string; // ios / android / windows / mac / devtools
  system?: string; // "iOS 16.6" / "Android 13"
  model?: string;
  SDKVersion?: string;
  [key: string]: unknown;
}

export interface WxApi {
  request(options: WxRequestOptions): void;
  getStorageSync(key: string): unknown;
  setStorageSync(key: string, value: unknown): void;
  removeStorageSync(key: string): void;
  getSystemInfoSync(): WxSystemInfo;
  onShow?(cb: () => void): void;
  onHide?(cb: () => void): void;
}

let injected: WxApi | undefined;

/** 测试/非标宿主注入 wx API 实现。 */
export function injectWx(api: WxApi): void {
  injected = api;
}

/** 取当前运行环境的 wx API；不在小程序/小游戏环境抛错。 */
export function getWx(): WxApi {
  if (injected) return injected;
  const g = globalThis as unknown as { wx?: WxApi };
  if (!g.wx || typeof g.wx.request !== 'function') {
    throw new Error('uba-wechat: 当前环境没有可用的 wx API，仅支持微信小程序/小游戏');
  }
  return g.wx;
}

/** 是否在小程序宿主（有 Page 全局）；否则视为小游戏。 */
export function isMiniProgramHost(): boolean {
  const g = globalThis as unknown as { Page?: unknown };
  return typeof g.Page === 'function';
}
