import type { TrackOptions } from './types';

export type { TrackOptions };

export interface UbaConfig {
  /** 应用 ID（管理后台「数据采集/应用管理」生成），必填。 */
  appId: string;
  /** 应用密钥，鉴权在请求 body 中完成，必填。 */
  appSecret: string;
  /** collector 服务地址，如 https://collector.example.com，必填（小程序需配置合法域名）。 */
  endpoint: string;
  /** 事件上报路径，默认 /uba/v1/report。 */
  path?: string;
  /** AB 开关拉取路径，默认 /uba/v1/flags。 */
  flagsPath?: string;
  /** 批量触发阈值，默认 20。 */
  batchSize?: number;
  /** 定时 flush 间隔（毫秒），默认 5000。 */
  flushInterval?: number;
  /** 失败重试次数（总尝试 = maxRetries+1），默认 3。 */
  maxRetries?: number;
  /** 单次请求超时（毫秒），必须 < 服务端 10s，默认 8000。 */
  timeout?: number;
  /** 重试退避基数（毫秒），默认 1000。 */
  retryBaseDelay?: number;
  /** 调试日志开关。 */
  debug?: boolean;
  /** 离线队列持久化开关（wx storage），默认 true。 */
  persistQueue?: boolean;
  /** 持久化队列最大条数，默认 500。 */
  maxPersistedEvents?: number;
  /** 单批请求体字符上限；服务端对未鉴权大包直接 401，必须压在 1MiB 内。 */
  maxBatchBytes?: number;
  /** 应用版本，随事件上报 behavior.appVersion。 */
  appVersion?: string;
  /** 渠道标识。 */
  channel?: string;
  /** 首次启动自动埋 app_launch（首装判定 = deviceId 新建），默认 false。 */
  autoTrackLaunch?: boolean;
  /** 页面浏览事件名（配合 instrumentPage / trackPageView），默认 page_view。 */
  pageViewEvent?: string;
  /** 会话轮换超时（毫秒）：后台超过该时长回到前台开启新会话，默认 30000。 */
  sessionTimeout?: number;
  /** 采样率 0~1，按 sessionId 确定性采样，默认 1。 */
  sampleRate?: number;
  /** 隐私合规：授权（setConsent(true)）前所有事件静默丢弃，默认 false。 */
  requireConsent?: boolean;
  /** 平台标识覆盖：默认自动探测（有 Page 全局 → mini_program，否则 mini_game）。 */
  platform?: 'mini_program' | 'mini_game' | string;
}

export interface ResolvedConfig {
  appId: string;
  appSecret: string;
  endpoint: string;
  path: string;
  flagsPath: string;
  batchSize: number;
  flushInterval: number;
  maxRetries: number;
  timeout: number;
  retryBaseDelay: number;
  debug: boolean;
  persistQueue: boolean;
  maxPersistedEvents: number;
  maxBatchBytes: number;
  appVersion?: string;
  channel?: string;
  autoTrackLaunch: boolean;
  pageViewEvent: string;
  sessionTimeout: number;
  sampleRate: number;
  requireConsent: boolean;
  platform?: string;

  reportUrl(): string;
  flagsUrl(): string;
}

/** 填默认值并校验（对齐其他端 SDK：批量 20/5s/3 次重试/1s 退避/8s 超时）。 */
export function resolveConfig(input: UbaConfig): ResolvedConfig {
  if (!input.appId) throw new Error('uba-wechat: appId 不能为空');
  if (!input.appSecret) throw new Error('uba-wechat: appSecret 不能为空');
  if (!input.endpoint) throw new Error('uba-wechat: endpoint 不能为空');
  const timeout = input.timeout ?? 8000;
  if (timeout <= 0 || timeout >= 10000) {
    throw new Error('uba-wechat: timeout 必须 > 0 且 < 10000（服务端超时 10s）');
  }
  const maxBatchBytes = input.maxBatchBytes ?? 900 * 1024;
  if (maxBatchBytes <= 0 || maxBatchBytes >= 1024 * 1024) {
    throw new Error('uba-wechat: maxBatchBytes 必须压在 1MiB 以内');
  }
  const endpoint = input.endpoint.replace(/\/+$/, '');
  return {
    appId: input.appId,
    appSecret: input.appSecret,
    endpoint,
    path: input.path ?? '/uba/v1/report',
    flagsPath: input.flagsPath ?? '/uba/v1/flags',
    batchSize: input.batchSize ?? 20,
    flushInterval: input.flushInterval ?? 5000,
    maxRetries: input.maxRetries ?? 3,
    timeout,
    retryBaseDelay: input.retryBaseDelay ?? 1000,
    debug: input.debug ?? false,
    persistQueue: input.persistQueue ?? true,
    maxPersistedEvents: input.maxPersistedEvents ?? 500,
    maxBatchBytes,
    appVersion: input.appVersion,
    channel: input.channel,
    autoTrackLaunch: input.autoTrackLaunch ?? false,
    pageViewEvent: input.pageViewEvent ?? 'page_view',
    sessionTimeout: input.sessionTimeout ?? 30000,
    sampleRate: input.sampleRate ?? 1,
    requireConsent: input.requireConsent ?? false,
    platform: input.platform,
    reportUrl: () => endpoint + (input.path ?? '/uba/v1/report'),
    flagsUrl: () => endpoint + (input.flagsPath ?? '/uba/v1/flags'),
  };
}
