/**
 * 会话回放录制器
 *
 * 基于 rrweb record()（动态导入，未安装 rrweb 时优雅降级），分块缓冲录制事件，
 * 定时/定量上传到 collector 的 /uba/v1/replays 端点。
 */

/** 每 chunk 最大事件数（超过即切块） */
const CHUNK_MAX_EVENTS = 100;
/** 定时上传间隔（毫秒） */
const FLUSH_INTERVAL_MS = 10_000;
/** 单事件 JSON 序列化失败保护 */
const MAX_CHUNK_BYTES = 2 << 20; // 2MB，给服务端 8MB 上限留余量

export interface ReplayUploaderOptions {
  endpoint: string;
  replaysPath?: string;
  appId: string;
  appSecret: string;
  sessionId: string;
  deviceId: string;
  userId?: number;
  pageUrl: string;
  debug?: boolean;
}

export class ReplayRecorder {
  private buffer: unknown[] = [];
  private timer: ReturnType<typeof setInterval> | null = null;
  private stopFn: (() => void) | undefined;
  private chunkSeq = 0;
  private startWallClock = 0;
  private opts: ReplayUploaderOptions;
  private log: (level: 'warn' | 'error' | 'info', msg: string) => void;

  constructor(opts: ReplayUploaderOptions, log: ReplayUploaderOptions['debug'] extends never ? never : (level: 'warn' | 'error' | 'info', msg: string) => void) {
    this.opts = opts;
    this.log = log;
  }

  /**
   * 启动录制：动态导入 rrweb（未安装返回 false，调用方感知降级）。
   */
  async start(): Promise<boolean> {
    try {
      const rrweb = await import('rrweb');
      this.startWallClock = Date.now();

      this.stopFn = rrweb.record({
        emit: (event: unknown) => {
          this.buffer.push(event);
          if (this.buffer.length >= CHUNK_MAX_EVENTS) {
            void this.flush();
          }
        },
      });

      if (!this.stopFn) {
        this.log('warn', 'rrweb record() returned null (browser may block recording)');
        return false;
      }

      this.timer = setInterval(() => void this.flush(), FLUSH_INTERVAL_MS);
      if (this.timer && typeof (this.timer as any).unref === 'function') {
        (this.timer as any).unref();
      }

      this.log('info', 'replay recording started');
      return true;
    } catch (err) {
      this.log('warn', `rrweb not available, replay disabled: ${err}`);
      return false;
    }
  }

  /** 停止录制并上传剩余事件。 */
  stop(): void {
    if (this.stopFn) {
      this.stopFn();
      this.stopFn = undefined;
    }
    if (this.timer) {
      clearInterval(this.timer);
      this.timer = null;
    }
    // 同步尽力上传剩余
    if (this.buffer.length > 0) {
      void this.flush();
    }
  }

  /** 上传一个分块。 */
  private async flush(): Promise<void> {
    if (this.buffer.length === 0) {
      return;
    }

    const events = this.buffer.splice(0, this.buffer.length);
    let json = '[]';
    try {
      json = JSON.stringify(events);
    } catch {
      this.log('warn', 'replay chunk serialize failed, dropped');
      return;
    }
    if (json.length > MAX_CHUNK_BYTES) {
      this.log('warn', `replay chunk ${json.length}B exceeds soft limit, dropped`);
      return;
    }

    const url = this.opts.endpoint + (this.opts.replaysPath || '/uba/v1/replays');
    try {
      const resp = await fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          appId: this.opts.appId,
          appSecret: this.opts.appSecret,
          sessionId: this.opts.sessionId,
          deviceId: this.opts.deviceId,
          userId: this.opts.userId,
          pageUrl: this.opts.pageUrl,
          chunkSeq: this.chunkSeq++,
          eventsJson: json,
          eventCount: events.length,
          durationMs: Date.now() - this.startWallClock,
        }),
      });
      if (!resp.ok) {
        this.log('warn', `replay chunk upload http ${resp.status}`);
      }
    } catch (err) {
      this.log('warn', `replay chunk upload failed: ${err}`);
    }
  }
}
