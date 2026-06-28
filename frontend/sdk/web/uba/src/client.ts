/**
 * UbaClient —— SDK 核心类
 *
 * 职责：
 * - 持有鉴权凭证（appId/appSecret）
 * - 提供 track / trackBehavior / trackRisk / identify / setSuperProperties / flush 等高层 API
 * - 自动补全公共字段（deviceId/sessionId/eventTime/platform/clientInfo/superProperties）
 * - 通过 Batcher 实现缓冲 + 批量上报
 */

import { Batcher } from './batcher';
import {
  getDeviceId,
  getSessionId,
  detectPlatform,
  getClientInfo,
  getPageUrl,
  isBrowser,
} from './context';
import { uuid, toRFC3339, merge } from './utils';
import {
  EventType,
  type UbaConfig,
  type ReportEvent,
  type TrackOptions,
  type ClientInfo,
  type PostReportResponse,
  type RiskEvent,
} from './types';

const DEFAULT_PATH = '/uba/v1/report';
const DEFAULT_BATCH_SIZE = 20;
const DEFAULT_FLUSH_INTERVAL = 5000;
const DEFAULT_MAX_RETRIES = 3;
const DEFAULT_TIMEOUT = 8000;
const DEFAULT_BASE_DELAY = 1000;

export class UbaClient {
  private config: Required<Pick<UbaConfig, 'appId' | 'appSecret' | 'endpoint' | 'path' | 'batchSize' | 'flushInterval' | 'maxRetries' | 'timeout' | 'retryBaseDelay' | 'debug' | 'enableBeacon'>>;
  private batcher: Batcher;

  /** 当前登录用户 ID（identify 设置后注入每条事件） */
  private userId?: number;
  /** 公共属性，自动注入每条事件 */
  private superProperties: Record<string, string> = {};
  /** 平台缓存 */
  private platform: string;

  private constructor(config: UbaConfig) {
    this.config = {
      appId: config.appId,
      appSecret: config.appSecret,
      endpoint: config.endpoint.replace(/\/+$/, ''), // 去尾部斜杠
      path: config.path || DEFAULT_PATH,
      batchSize: config.batchSize ?? DEFAULT_BATCH_SIZE,
      flushInterval: config.flushInterval ?? DEFAULT_FLUSH_INTERVAL,
      maxRetries: config.maxRetries ?? DEFAULT_MAX_RETRIES,
      timeout: config.timeout ?? DEFAULT_TIMEOUT,
      retryBaseDelay: config.retryBaseDelay ?? DEFAULT_BASE_DELAY,
      debug: config.debug ?? false,
      enableBeacon: config.enableBeacon ?? true,
    };

    this.platform = detectPlatform();
    const url = this.config.endpoint + this.config.path;

    this.batcher = new Batcher({
      appId: this.config.appId,
      appSecret: this.config.appSecret,
      url,
      batchSize: this.config.batchSize,
      flushInterval: this.config.flushInterval,
      maxRetries: this.config.maxRetries,
      timeout: this.config.timeout,
      baseDelay: this.config.retryBaseDelay,
      enableBeacon: this.config.enableBeacon,
      getClientInfo: () => this.resolveClientInfo(),
      log: (level, msg) => this.log(level, msg),
    });

    // 绑定页面卸载兜底
    if (isBrowser() && this.config.enableBeacon) {
      this.bindUnload();
    }
  }

  /**
   * 初始化 SDK（单例）。
   * 多次调用以最后一次为准（重新创建 batcher）。
   */
  static init(config: UbaConfig): UbaClient {
    if (!config.appId || !config.appSecret) {
      throw new Error('UbaClient.init: appId and appSecret are required');
    }
    if (!config.endpoint) {
      throw new Error('UbaClient.init: endpoint is required');
    }
    // 单例：复用全局实例
    const g = globalThis as any;
    if (g.__uba_client__ instanceof UbaClient) {
      g.__uba_client__.destroy();
    }
    const client = new UbaClient(config);
    g.__uba_client__ = client;
    return client;
  }

  /** 获取单例（init 之后可用） */
  static getInstance(): UbaClient | undefined {
    return (globalThis as any).__uba_client__ as UbaClient | undefined;
  }

  /**
   * 通用埋点：上报一个行为事件。
   * @param eventName 事件名称（必填）
   * @param properties 自定义属性，并入 properties 字段
   * @param options 其它字段（userId/objectType/amount/metrics 等）
   */
  track(eventName: string, properties?: Record<string, string>, options?: TrackOptions): void {
    this.trackBehavior(eventName, properties, options);
  }

  /** 行为事件埋点（显式语义） */
  trackBehavior(eventName: string, properties?: Record<string, string>, options?: TrackOptions): void {
    const event = this.buildEvent(EventType.Behavior, eventName, properties, options);
    // 构造 behavior oneof payload（与顶层业务字段对齐，便于服务端落库）
    event.behavior = {
      eventAction: options?.eventAction,
      objectType: options?.objectType,
      objectId: options?.objectId,
      objectName: options?.objectName,
      durationMs: options?.durationMs,
      amount: options?.amount,
      quantity: options?.quantity,
      score: options?.score,
      metrics: options?.metrics,
    };
    this.enqueue(event);
  }

  /** 风险事件埋点 */
  trackRisk(eventName: string, risk: Partial<RiskEvent>, options?: TrackOptions): void {
    const event = this.buildEvent(EventType.Risk, eventName, options?.properties, options);
    // 构造 risk oneof payload
    event.risk = {
      ...risk,
    };
    this.enqueue(event);
  }

  /** 设置当前登录用户 ID，后续事件自动带上 */
  identify(userId: number): void {
    this.userId = userId;
  }

  /** 清除登录用户（登出） */
  resetUser(): void {
    this.userId = undefined;
  }

  /** 设置公共属性，注入后续每条事件的 properties */
  setSuperProperties(props: Record<string, string>): void {
    this.superProperties = merge(this.superProperties, props);
  }

  /** 清除公共属性 */
  clearSuperProperties(): void {
    this.superProperties = {};
  }

  /** 手动触发批量上报 */
  async flush(): Promise<void> {
    await this.batcher.flush();
  }

  /** 销毁 SDK：停止定时器。通常无需手动调用。 */
  destroy(): void {
    this.batcher.destroy();
    const g = globalThis as any;
    if (g.__uba_client__ === this) {
      g.__uba_client__ = undefined;
    }
  }

  /** 当前队列中待上报的事件数 */
  pendingCount(): number {
    return this.batcher.size();
  }

  // ──────────── 内部方法 ────────────

  /** 构造一个 ReportEvent，自动补全公共字段 */
  private buildEvent(
    eventType: EventType,
    eventName: string,
    properties?: Record<string, string>,
    options?: TrackOptions,
  ): ReportEvent {
    const eventTime = toRFC3339();
    // 合并 properties：superProperties（基础）+ 本次传入
    const mergedProps = merge(this.superProperties, properties);
    // 注入 pageUrl 便于路径分析（若在浏览器且有值）
    const pageUrl = getPageUrl();
    if (pageUrl && !mergedProps.pageUrl) {
      mergedProps.pageUrl = pageUrl;
    }

    return {
      eventType,
      eventId: uuid(),
      eventName,
      eventTime,
      userId: options?.userId ?? this.userId,
      deviceId: options?.deviceId ?? getDeviceId(),
      sessionId: options?.sessionId ?? getSessionId(),
      platform: options?.platform ?? this.platform,
      eventCategory: options?.eventCategory,
      eventAction: options?.eventAction,
      objectType: options?.objectType,
      objectId: options?.objectId,
      objectName: options?.objectName,
      durationMs: options?.durationMs,
      amount: options?.amount,
      quantity: options?.quantity,
      score: options?.score,
      metrics: options?.metrics,
      properties: Object.keys(mergedProps).length > 0 ? mergedProps : undefined,
    };
  }

  private enqueue(event: ReportEvent): void {
    this.batcher.enqueue(event);
    if (this.config.debug) {
      this.log('info', `enqueued event: ${event.eventName} (queue=${this.batcher.size()})`);
    }
  }

  private resolveClientInfo(): ClientInfo | undefined {
    return getClientInfo();
  }

  private bindUnload(): void {
    const handler = () => this.batcher.flushBeacon();
    // visibilitychange（移动端/现代浏览器）+ beforeunload 双保险
    window.addEventListener('pagehide', handler);
    window.addEventListener('beforeunload', handler);
  }

  private log(level: 'warn' | 'error' | 'info', msg: string): void {
    if (!this.config.debug && level !== 'error') {
      return;
    }
    const prefix = '[UBA SDK]';
    // eslint-disable-next-line no-console
    const fn = (console as any)[level === 'info' ? 'log' : level] || console.log;
    try {
      fn.call(console, prefix, msg);
    } catch {
      // console 不可用时忽略
    }
  }
}

/** 重新导出响应类型，便于调用方处理 flush 结果 */
export type { PostReportResponse };
