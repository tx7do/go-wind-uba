import type { ResolvedConfig } from './config';
import { OFFLINE_QUEUE_KEY, storageGet, storageRemove, storageSet } from './storage';
import { requestJson } from './transport';
import { putIfPresent } from './types';
import type { ClientInfo, FlushResult, PostReportResponse, ReportEvent } from './types';

type Logger = (message: string) => void;

/**
 * 队列 + 批量上报器：
 * - 内存队列，达到 batchSize 或定时器到期即 flush；
 * - 重试策略对齐 C#/Flutter SDK：指数退避，4xx（除 429）不重试；
 * - 可重试失败（网络/5xx/429）落盘，下次启动补发；确定性 4xx 直接丢弃。
 */
export class Batcher {
  private queue: ReportEvent[] = [];
  private timer: ReturnType<typeof setInterval> | null = null;
  private flushing = false;
  private closed = false;

  constructor(
    private readonly config: ResolvedConfig,
    private readonly clientInfoProvider: () => ClientInfo | undefined,
    private readonly log: Logger,
  ) {}

  size(): number {
    return this.queue.length;
  }

  start(): void {
    if (this.config.flushInterval <= 0 || this.timer !== null) return;
    this.timer = setInterval(() => {
      if (this.queue.length > 0) void this.flush();
    }, this.config.flushInterval);
    // 小程序定时器不阻塞退出，无需 unref 处理
  }

  stop(): void {
    if (this.timer !== null) {
      clearInterval(this.timer);
      this.timer = null;
    }
  }

  enqueue(event: ReportEvent): void {
    if (this.closed) return;
    this.queue.push(event);
    if (this.queue.length >= this.config.batchSize) {
      void this.flush();
    }
  }

  /** 恢复落盘的离线队列（最旧优先发送）。 */
  restorePersisted(): void {
    if (!this.config.persistQueue) return;
    const raw = storageGet(OFFLINE_QUEUE_KEY);
    if (!raw) return;
    try {
      const list = JSON.parse(raw) as unknown;
      if (!Array.isArray(list)) throw new Error('not a list');
      const restored = list.filter(isRecord).map(toEvent);
      this.queue.unshift(...restored);
      storageRemove(OFFLINE_QUEUE_KEY);
      this.log(`离线队列恢复 ${restored.length} 条`);
    } catch {
      this.log('离线队列解析失败，已忽略');
      storageRemove(OFFLINE_QUEUE_KEY);
    }
  }

  /** 手动 flush：分批发送直到清空队列或遇到失败。并发调用安全。 */
  async flush(): Promise<FlushResult> {
    if (this.flushing || this.closed) {
      return { success: true, sent: 0, dropped: 0 };
    }
    this.flushing = true;
    let sent = 0;
    let dropped = 0;
    let ok = true;
    let retryable = false;
    let message: string | undefined;
    try {
      while (this.queue.length > 0) {
        const batch = this.takeBatch();
        const result = await this.sendWithRetry(batch);
        sent += result.sent;
        dropped += result.dropped;
        if (!result.success) {
          ok = false;
          message = result.message;
          retryable = result.retryable === true;
          if (retryable) this.queue.unshift(...batch);
          break;
        }
      }
      this.persistQueue();
    } finally {
      this.flushing = false;
    }
    return { success: ok, sent, dropped, retryable, message };
  }

  close(): void {
    this.closed = true;
    this.stop();
  }

  /** 取一批：受 batchSize 与 maxBatchBytes 双重限制（按 JSON 字符数近似字节数）。 */
  private takeBatch(): ReportEvent[] {
    const batch: ReportEvent[] = [];
    let bytes = 0;
    while (this.queue.length > 0 && batch.length < this.config.batchSize) {
      const encoded = JSON.stringify(toWire(this.queue[0]));
      if (batch.length > 0 && bytes + encoded.length > this.config.maxBatchBytes) break;
      bytes += encoded.length;
      batch.push(this.queue.shift() as ReportEvent);
    }
    return batch;
  }

  private async sendWithRetry(batch: ReportEvent[]): Promise<FlushResult> {
    const envelope: Record<string, unknown> = {
      appId: this.config.appId,
      appSecret: this.config.appSecret,
      events: batch.map(toWire),
    };
    const clientInfo = this.clientInfoProvider();
    putIfPresent(envelope, 'clientInfo', clientInfo ? clientInfoToWire(clientInfo) : undefined);
    const body = JSON.stringify(envelope);

    for (let attempt = 0; ; attempt++) {
      const res = await requestJson(this.config.reportUrl(), body, this.config.timeout);

      if (res.statusCode >= 200 && res.statusCode < 300) {
        let failed = 0;
        if (isRecord(res.body)) {
          const parsed = res.body as PostReportResponse;
          failed = typeof parsed.failedCount === 'number' ? parsed.failedCount : 0;
          if (failed > 0) this.log(`部分事件被服务端拒绝：${String(parsed.message ?? '')}`);
        } else {
          this.log(`响应不是 JSON（HTTP ${res.statusCode}），按整批成功处理`);
        }
        return { success: true, sent: batch.length - failed, dropped: failed };
      }

      const status = res.statusCode;
      // 4xx（除 429）是确定性失败：整批丢弃，不重试
      if (status >= 400 && status < 500 && status !== 429) {
        this.log(`HTTP ${status}（不可重试），丢弃 ${batch.length} 条`);
        return {
          success: false,
          sent: 0,
          dropped: batch.length,
          retryable: false,
          message: `HTTP ${status}`,
        };
      }

      if (attempt >= this.config.maxRetries) {
        this.log(`重试耗尽（HTTP ${status}），${batch.length} 条待落盘补发`);
        return {
          success: false,
          sent: 0,
          dropped: batch.length,
          retryable: true,
          message: `HTTP ${status} after ${attempt + 1} attempts`,
        };
      }

      const delay = this.config.retryBaseDelay * 2 ** attempt;
      this.log(`HTTP ${status}，${delay}ms 后第 ${attempt + 1} 次重试`);
      await sleep(delay);
    }
  }

  private persistQueue(): void {
    if (!this.config.persistQueue) return;
    if (this.queue.length > this.config.maxPersistedEvents) {
      this.queue = this.queue.slice(this.queue.length - this.config.maxPersistedEvents);
    }
    if (this.queue.length === 0) {
      storageRemove(OFFLINE_QUEUE_KEY);
      return;
    }
    storageSet(OFFLINE_QUEUE_KEY, JSON.stringify(this.queue.map(toWire)));
  }
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === 'object' && v !== null && !Array.isArray(v);
}

/** ReportEvent → 传输 JSON（跳过 undefined，数值标量不落顶层已在构造时保证）。 */
function toWire(e: ReportEvent): Record<string, unknown> {
  const m: Record<string, unknown> = {
    eventType: e.eventType,
    eventId: e.eventId,
    eventName: e.eventName,
    eventTime: e.eventTime,
  };
  putIfPresent(m, 'userId', e.userId);
  putIfPresent(m, 'deviceId', e.deviceId);
  putIfPresent(m, 'sessionId', e.sessionId);
  putIfPresent(m, 'platform', e.platform);
  putIfPresent(m, 'eventCategory', e.eventCategory);
  putIfPresent(m, 'properties', e.properties);
  putIfPresent(m, 'metrics', e.metrics);
  putIfPresent(m, 'traceId', e.traceId);
  putIfPresent(m, 'eventAction', e.eventAction);
  putIfPresent(m, 'objectType', e.objectType);
  putIfPresent(m, 'objectId', e.objectId);
  putIfPresent(m, 'objectName', e.objectName);
  putIfPresent(m, 'amount', e.amount);
  putIfPresent(m, 'opResult', e.opResult);
  putIfPresent(m, 'errorCode', e.errorCode);
  if (e.behavior) putIfPresent(m, 'behavior', toBehaviorWire(e.behavior));
  if (e.risk) putIfPresent(m, 'risk', toRecordWire(e.risk));
  return m;
}

function toBehaviorWire(b: NonNullable<ReportEvent['behavior']>): Record<string, unknown> {
  const m: Record<string, unknown> = {};
  putIfPresent(m, 'eventAction', b.eventAction);
  putIfPresent(m, 'objectType', b.objectType);
  putIfPresent(m, 'objectId', b.objectId);
  putIfPresent(m, 'objectName', b.objectName);
  putIfPresent(m, 'sessionSeq', b.sessionSeq);
  putIfPresent(m, 'os', b.os);
  putIfPresent(m, 'appVersion', b.appVersion);
  putIfPresent(m, 'channel', b.channel);
  putIfPresent(m, 'network', b.network);
  putIfPresent(m, 'durationMs', b.durationMs);
  putIfPresent(m, 'amount', b.amount);
  putIfPresent(m, 'quantity', b.quantity);
  putIfPresent(m, 'score', b.score);
  putIfPresent(m, 'metrics', b.metrics);
  putIfPresent(m, 'properties', b.properties);
  putIfPresent(m, 'opResult', b.opResult);
  putIfPresent(m, 'errorCode', b.errorCode);
  putIfPresent(m, 'clickX', b.clickX);
  putIfPresent(m, 'clickY', b.clickY);
  putIfPresent(m, 'elementXpath', b.elementXpath);
  putIfPresent(m, 'pageUrl', b.pageUrl);
  putIfPresent(m, 'viewportWidth', b.viewportWidth);
  putIfPresent(m, 'serverId', b.serverId);
  putIfPresent(m, 'level', b.level);
  return m;
}

function toRecordWire(v: object): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const [k, val] of Object.entries(v)) {
    putIfPresent(out, k, val);
  }
  return out;
}

function clientInfoToWire(c: ClientInfo): Record<string, unknown> {
  const m: Record<string, unknown> = {};
  putIfPresent(m, 'userAgent', c.userAgent);
  putIfPresent(m, 'referer', c.referer);
  putIfPresent(m, 'country', c.country);
  putIfPresent(m, 'city', c.city);
  return m;
}

function toEvent(v: Record<string, unknown>): ReportEvent {
  return {
    eventType: (v['eventType'] === 'RISK' ? 'RISK' : 'BEHAVIOR') as ReportEvent['eventType'],
    eventId: str(v['eventId']) ?? '',
    eventName: str(v['eventName']) ?? '',
    eventTime: str(v['eventTime']) ?? '',
    userId: num(v['userId']),
    deviceId: str(v['deviceId']),
    sessionId: str(v['sessionId']),
    platform: str(v['platform']),
    eventCategory: str(v['eventCategory']),
    properties: v['properties'] as ReportEvent['properties'],
    metrics: v['metrics'] as ReportEvent['metrics'],
    traceId: str(v['traceId']),
    eventAction: str(v['eventAction']),
    objectType: str(v['objectType']),
    objectId: str(v['objectId']),
    objectName: str(v['objectName']),
    amount: str(v['amount']),
    opResult: str(v['opResult']),
    errorCode: str(v['errorCode']),
    behavior: v['behavior'] as ReportEvent['behavior'],
    risk: v['risk'] as ReportEvent['risk'],
  };
}

function str(v: unknown): string | undefined {
  return typeof v === 'string' ? v : undefined;
}

function num(v: unknown): number | undefined {
  return typeof v === 'number' ? v : undefined;
}
