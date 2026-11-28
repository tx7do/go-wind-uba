/**
 * 事件缓冲与批量合并
 *
 * 内存队列缓冲事件，达到 batchSize 或 flushInterval 触发批量上报。
 * 上报失败且重试耗尽时，事件被丢弃（避免内存无限增长）。
 */

import { sendWithRetry, sendBeacon, type FetchResult } from './retry';
import type { ReportEvent, PostReportRequest, PostReportResponse, ClientInfo } from './types';
import {
  loadPersistedEvents,
  persistEvents,
  clearPersistedEvents,
} from './storage';
import { isBrowser } from './context';

export interface BatcherOptions {
  appId: string;
  appSecret: string;
  /** 完整上报 URL */
  url: string;
  /** 批量大小阈值 */
  batchSize: number;
  /** 定时 flush 间隔（毫秒） */
  flushInterval: number;
  /** 透传给 sendWithRetry 的配置 */
  maxRetries: number;
  timeout: number;
  baseDelay: number;
  /** 是否启用 beacon 兜底 */
  enableBeacon: boolean;
  /** 是否把未上报队列持久化到 localStorage（崩溃/断网后补发） */
  persistQueue: boolean;
  /** 持久化队列上限（条） */
  maxPersistedEvents: number;
  /** clientInfo 注入器（每次 flush 取最新值） */
  getClientInfo: () => ClientInfo | undefined;
  /** 日志 */
  log: (level: 'warn' | 'error' | 'info', msg: string) => void;
}

export interface FlushResult {
  success: boolean;
  response?: PostReportResponse;
  /** 丢弃的事件数（重试耗尽后） */
  dropped: number;
}

export class Batcher {
  private queue: ReportEvent[] = [];
  private timer: ReturnType<typeof setTimeout> | null = null;
  private flushing = false;
  /** 持久化写盘节流句柄 */
  private persistTimer: ReturnType<typeof setTimeout> | null = null;

  constructor(private opts: BatcherOptions) {
    // 启动时回收上次未上报完的事件（崩溃/断网补发）
    if (this.opts.persistQueue && isBrowser()) {
      const restored = loadPersistedEvents();
      if (restored.length > 0) {
        this.queue.push(...restored);
        clearPersistedEvents();
        this.opts.log('info', `restored ${restored.length} offline events`);
      }
    }
    this.startTimer();
  }

  /** 入队一条事件，满 batchSize 自动 flush */
  enqueue(event: ReportEvent): void {
    this.queue.push(event);
    this.schedulePersist();
    if (this.queue.length >= this.opts.batchSize) {
      void this.flush();
    }
  }

  /** 队列镜像到 localStorage（节流 500ms，避免高频写盘） */
  private schedulePersist(): void {
    if (!this.opts.persistQueue || !isBrowser() || this.persistTimer) {
      return;
    }
    this.persistTimer = setTimeout(() => {
      this.persistTimer = null;
      persistEvents(this.queue, this.opts.maxPersistedEvents);
    }, 500);
    if (this.persistTimer && typeof (this.persistTimer as any).unref === 'function') {
      (this.persistTimer as any).unref();
    }
  }

  /** 当前队列长度 */
  size(): number {
    return this.queue.length;
  }

  /**
   * 触发批量上报。从队列取出当前所有事件发送。
   * 多次并发调用只会执行一次实际发送（其余等待）。
   */
  async flush(): Promise<FlushResult> {
    if (this.flushing) {
      return { success: true, dropped: 0 };
    }
    if (this.queue.length === 0) {
      return { success: true, dropped: 0 };
    }

    this.flushing = true;
    const events = this.queue.splice(0, this.queue.length);

    try {
      const body = this.buildBody(events);
      const result = await sendWithRetry(this.opts.url, body, {
        maxRetries: this.opts.maxRetries,
        timeout: this.opts.timeout,
        baseDelay: this.opts.baseDelay,
        log: (level, msg) => this.opts.log(level, msg),
      });

      if (result.ok) {
        // 注意：HTTP 200 也可能含部分失败事件（errorsByType），这里记录但不视为整体失败
        const resp = result.response;
        if (resp && resp.failedCount && resp.failedCount > 0) {
          this.opts.log(
            'warn',
            `upload partial failure: success=${resp.successCount} failed=${resp.failedCount}`,
          );
        }
        // 整体发送成功：清掉离线镜像
        if (this.opts.persistQueue) {
          clearPersistedEvents();
        }
        return { success: true, response: resp, dropped: 0 };
      }

      // 发送失败（含重试耗尽）：事件写回离线镜像，下次启动补发
      if (this.opts.persistQueue) {
        persistEvents(events, this.opts.maxPersistedEvents);
        this.opts.log('warn', `persisted ${events.length} events for offline retry`);
      } else {
        this.opts.log('error', `dropping ${events.length} events due to upload failure`);
      }
      return { success: false, dropped: this.opts.persistQueue ? 0 : events.length };
    } finally {
      this.flushing = false;
    }
  }

  /**
   * 页面卸载兜底：用 sendBeacon 尽力投递当前队列。
   * 不等待响应，best-effort。
   */
  flushBeacon(): void {
    if (!this.opts.enableBeacon || this.queue.length === 0) {
      return;
    }
    const events = this.queue.splice(0, this.queue.length);
    const body = this.buildBody(events);
    const ok = sendBeacon(this.opts.url, body);
    if (!ok) {
      // beacon 失败：写回离线镜像，下次打开补发
      if (this.opts.persistQueue) {
        persistEvents(events, this.opts.maxPersistedEvents);
      }
      this.opts.log('warn', 'sendBeacon failed, events persisted offline');
    }
  }

  /** 销毁：停止定时器，把未上报事件落盘 */
  destroy(): void {
    this.stopTimer();
    if (this.persistTimer) {
      clearTimeout(this.persistTimer);
      this.persistTimer = null;
    }
    if (this.opts.persistQueue && this.queue.length > 0) {
      persistEvents(this.queue, this.opts.maxPersistedEvents);
    }
  }

  private buildBody(events: ReportEvent[]): string {
    const req: PostReportRequest = {
      appId: this.opts.appId,
      appSecret: this.opts.appSecret,
      events,
      clientInfo: this.opts.getClientInfo(),
    };
    return JSON.stringify(req);
  }

  private startTimer(): void {
    this.stopTimer();
    this.timer = setInterval(() => {
      void this.flush();
    }, this.opts.flushInterval);
    // Node 下 setInterval 返回 NodeJS.Timeout，浏览器返回 number；不阻塞进程退出
    if (this.timer && typeof (this.timer as any).unref === 'function') {
      (this.timer as any).unref();
    }
  }

  private stopTimer(): void {
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = null;
    }
  }
}

/** 重新导出，便于上层使用 */
export type { FetchResult };
