import { resolveConfig } from './config';
import type { ResolvedConfig, UbaConfig } from './config';
import { loadContext } from './context';
import type { ContextSnapshot } from './context';
import { Batcher } from './batcher';
import { SessionManager } from './session';
import { requestJson } from './transport';
import { mergeProperties, toRFC3339Ms, uuidV4 } from './utils';
import type {
  ClientInfo,
  EventType,
  FlushResult,
  ReportEvent,
  RiskEvent,
  TrackOptions,
} from './types';

let instance: UbaClient | undefined;

/**
 * GoWind UBA 微信小程序/小游戏埋点客户端（单例）。
 *
 * 用法（app.ts / app.js）：
 * ```ts
 * import { UbaClient } from '@go-wind-uba/uba-wechat-sdk';
 *
 * const uba = UbaClient.init({
 *   appId: 'your_app_id',
 *   appSecret: 'your_app_secret',
 *   endpoint: 'https://collector.example.com',
 * });
 * App(uba.instrumentApp({ onLaunch() {...} }));
 * Page(uba.instrumentPage({ onLoad() {...} }));
 * ```
 *
 * 方法名与 Web / C# / Flutter / Go SDK 对齐。
 */
export class UbaClient {
  private readonly config: ResolvedConfig;
  private readonly context: ContextSnapshot;
  private readonly batcher: Batcher;
  private readonly session: SessionManager;

  private userId?: number;
  private readonly superProps: Record<string, string> = {};
  private consentGranted = false;
  private closed = false;
  private lastScreenAt: number | null = null;
  private readonly exposedFlags = new Set<string>();
  private exposedSessionId: string | null = null;
  private flags: Record<string, string> = {};

  private constructor(config: ResolvedConfig, context: ContextSnapshot) {
    this.config = config;
    this.context = context;
    this.session = new SessionManager(config.sessionTimeout);
    this.batcher = new Batcher(config, () => this.clientInfo(), (msg) => this.debugLog(msg));
  }

  /** 初始化全局单例（重复调用会关闭并替换旧实例）。 */
  static init(config: UbaConfig): UbaClient {
    const resolved = resolveConfig(config);
    const context = loadContext(resolved.platform);
    if (instance) instance.destroy();
    instance = new UbaClient(resolved, context);
    instance.batcher.restorePersisted();
    instance.batcher.start();
    if (resolved.autoTrackLaunch && context.deviceIdIsNew) {
      instance.track('app_launch', { first_launch: 'true' }, { eventCategory: 'app' });
    }
    return instance;
  }

  /** 当前全局实例。 */
  static getInstance(): UbaClient | undefined {
    return instance;
  }

  // ─── 上报 API ────────────────────────────────────────────────────────────

  /** 上报自定义行为事件（trackBehavior 别名）。 */
  track(
    eventName: string,
    properties?: Record<string, string>,
    options?: TrackOptions,
  ): void {
    this.trackBehavior(eventName, properties, options);
  }

  /** 上报自定义行为事件。 */
  trackBehavior(
    eventName: string,
    properties?: Record<string, string>,
    options?: TrackOptions,
  ): void {
    const event = this.buildBehaviorEvent(eventName, properties, options);
    if (!event) return;
    this.batcher.enqueue(event);
  }

  /** 上报风险事件（进入 uba_risk_events 通道）。 */
  trackRisk(
    eventName: string,
    risk: RiskEvent,
    properties?: Record<string, string>,
    options?: TrackOptions,
  ): void {
    if (!this.shouldCollect(eventName)) return;
    this.batcher.enqueue({
      eventType: 'RISK' as EventType,
      eventId: uuidV4(),
      eventName,
      eventTime: toRFC3339Ms(new Date()),
      userId: options?.userId ?? this.userId,
      deviceId: options?.deviceId ?? this.context.deviceId,
      sessionId: options?.sessionId ?? this.session.current(),
      platform: options?.platform ?? this.context.platform,
      eventCategory: options?.eventCategory,
      properties: this.mergedProperties(properties, options),
      traceId: options?.traceId,
      risk,
    });
  }

  /** 页面浏览事件；自动携带上一页面停留时长 durationMs。 */
  trackPageView(screenName: string, title?: string, referer?: string): void {
    const now = Date.now();
    const dwell = this.lastScreenAt === null ? undefined : now - this.lastScreenAt;
    this.lastScreenAt = now;
    const props: Record<string, string> = { pageUrl: screenName };
    if (title) props.title = title;
    if (referer) props.referer = referer;
    this.trackBehavior(this.config.pageViewEvent, props, {
      eventCategory: 'navigation',
      objectType: 'page',
      objectId: screenName,
      durationMs: dwell !== undefined && dwell > 0 ? dwell : undefined,
    });
  }

  /** 登录绑定用户 ID（小程序内登录态）。 */
  identify(userId: number): void {
    this.userId = userId;
  }

  /** 登出清空用户绑定。 */
  resetUser(): void {
    this.userId = undefined;
  }

  /** 设置全局公共属性：合并进之后每个事件（同名覆盖）。 */
  setSuperProperties(props: Record<string, string>): void {
    Object.assign(this.superProps, props);
  }

  /** 清空全局公共属性。 */
  clearSuperProperties(): void {
    for (const key of Object.keys(this.superProps)) delete this.superProps[key];
  }

  /** 隐私合规授权开关；requireConsent = true 时未授权前所有上报静默丢弃。 */
  setConsent(granted: boolean): void {
    this.consentGranted = granted;
  }

  /** 当前是否处于采集状态（未关闭、consent 放行、会话命中采样）。 */
  isCollecting(): boolean {
    return !this.closed && this.consentOk() && this.inSample();
  }

  // ─── 生命周期（instrumentApp 自动调用，也可手动接 wx onHide/onShow）───────

  /** 退到后台：立即 flush + 记录时刻。 */
  handleBackground(): void {
    this.session.onBackground();
    void this.batcher.flush();
  }

  /** 回到前台：后台超过 sessionTimeout 则轮换新会话。 */
  handleForeground(): void {
    this.session.onForeground();
  }

  // ─── AB 实验开关 ──────────────────────────────────────────────────────────

  /** 拉取最新实验开关并缓存。失败保留旧缓存。 */
  async loadFlags(): Promise<Record<string, string>> {
    const body: Record<string, unknown> = {
      appId: this.config.appId,
      appSecret: this.config.appSecret,
      deviceId: this.context.deviceId,
    };
    if (this.userId != null) body.userId = this.userId;
    const res = await requestJson(
      this.config.flagsUrl(),
      JSON.stringify(body),
      this.config.timeout,
    );
    if (res.statusCode >= 200 && res.statusCode < 300) {
      const data = res.body as { flags?: Record<string, string> } | undefined;
      if (data && typeof data === 'object' && data.flags) {
        this.flags = { ...data.flags };
      } else {
        this.flags = {};
      }
    } else {
      this.debugLog(`拉取 flags 失败：HTTP ${res.statusCode}`);
    }
    return { ...this.flags };
  }

  /** 读取开关变体；命中时自动上报一次 $feature_flag 曝光（每会话每开关一次）。 */
  getFlag(key: string): string | undefined {
    const variant = this.flags[key];
    if (variant !== undefined) this.trackFlagExposure(key, variant);
    return variant;
  }

  /** 变体非空且非 control 即视为命中（不触发曝光事件）。 */
  isFeatureEnabled(key: string): boolean {
    const v = this.flags[key];
    return v !== undefined && v !== '' && v !== 'control';
  }

  // ─── 自动埋点包装器 ───────────────────────────────────────────────────────

  /**
   * 包装 App() 配置：onHide/onShow 自动接后台 flush 与会话轮换。
   * `App(uba.instrumentApp({ ... }))`
   */
  instrumentApp<T extends Record<string, unknown>>(appConfig: T): T {
    const client = this;
    const originalShow = appConfig['onShow'];
    const originalHide = appConfig['onHide'];
    return {
      ...appConfig,
      onShow(this: unknown, ...args: unknown[]) {
        client.handleForeground();
        if (typeof originalShow === 'function') {
          (originalShow as (...a: unknown[]) => void).apply(this, args);
        }
      },
      onHide(this: unknown, ...args: unknown[]) {
        client.handleBackground();
        if (typeof originalHide === 'function') {
          (originalHide as (...a: unknown[]) => void).apply(this, args);
        }
      },
    } as unknown as T;
  }

  /**
   * 包装 Page() 配置：onLoad 自动上报 page_view（含来源页与停留时长）。
   * `Page(uba.instrumentPage({ ... }))`
   */
  instrumentPage<T extends Record<string, unknown>>(pageConfig: T): T {
    const client = this;
    const originalLoad = pageConfig['onLoad'];
    return {
      ...pageConfig,
      onLoad(this: { route?: string }, ...args: unknown[]) {
        const route = typeof this?.route === 'string' ? this.route : '';
        const referer = client.currentScreen;
        if (route) client.trackPageView(route, undefined, referer ?? undefined);
        if (route) client.currentScreen = route;
        if (typeof originalLoad === 'function') {
          (originalLoad as (...a: unknown[]) => void).apply(this, args);
        }
      },
    } as unknown as T;
  }

  // ─── 维护 ────────────────────────────────────────────────────────────────

  /** 手动 flush 全部队列。 */
  async flush(): Promise<FlushResult> {
    return this.batcher.flush();
  }

  /** 队列中待上报条数。 */
  pendingCount(): number {
    return this.batcher.size();
  }

  /** 关闭：停止定时器与自动上报（离线队列保留，下次启动补发）。 */
  destroy(): void {
    this.closed = true;
    this.batcher.close();
    if (instance === this) instance = undefined;
  }

  // ─── 内部实现 ────────────────────────────────────────────────────────────

  private currentScreen: string | null = null;

  private clientInfo(): ClientInfo {
    return { userAgent: this.context.userAgent };
  }

  private consentOk(): boolean {
    return !this.config.requireConsent || this.consentGranted;
  }

  private inSample(): boolean {
    return this.session.inSample(this.config.sampleRate);
  }

  private shouldCollect(eventName: string): boolean {
    if (this.closed) return false;
    if (!this.consentOk()) {
      this.debugLog(`consent 未授权，丢弃事件 ${eventName}`);
      return false;
    }
    return this.inSample();
  }

  private buildBehaviorEvent(
    eventName: string,
    properties?: Record<string, string>,
    options?: TrackOptions,
  ): ReportEvent | undefined {
    if (!this.shouldCollect(eventName)) return undefined;
    const opts = options ?? {};
    const behavior: NonNullable<ReportEvent['behavior']> = {
      eventAction: opts.eventAction,
      objectType: opts.objectType,
      objectId: opts.objectId,
      objectName: opts.objectName,
      os: this.context.os,
      appVersion: this.config.appVersion,
      channel: this.config.channel,
      network: opts.network,
      durationMs: opts.durationMs,
      amount: opts.amount,
      quantity: opts.quantity,
      score: opts.score,
      metrics: opts.metrics,
      opResult: opts.opResult,
      errorCode: opts.errorCode,
      serverId: opts.serverId,
      level: opts.level,
    };
    return {
      eventType: 'BEHAVIOR' as EventType,
      eventId: uuidV4(),
      eventName,
      eventTime: toRFC3339Ms(new Date()),
      userId: opts.userId ?? this.userId,
      deviceId: opts.deviceId ?? this.context.deviceId,
      sessionId: opts.sessionId ?? this.session.current(),
      platform: opts.platform ?? this.context.platform,
      eventCategory: opts.eventCategory,
      properties: this.mergedProperties(properties, opts),
      metrics: opts.metrics,
      traceId: opts.traceId,
      // 顶层字符串镜像维度；数值标量只进 behavior（proto3 无法区分 unset 0）
      eventAction: opts.eventAction,
      objectType: opts.objectType,
      objectId: opts.objectId,
      objectName: opts.objectName,
      amount: opts.amount,
      opResult: opts.opResult,
      errorCode: opts.errorCode,
      behavior,
    };
  }

  private mergedProperties(
    properties?: Record<string, string>,
    options?: TrackOptions,
  ): Record<string, string> | undefined {
    const merged = mergeProperties(this.superProps, properties);
    if (options?.properties) Object.assign(merged, options.properties);
    return Object.keys(merged).length > 0 ? merged : undefined;
  }

  private trackFlagExposure(key: string, variant: string): void {
    const sessionId = this.session.current();
    if (this.exposedSessionId !== sessionId) {
      this.exposedFlags.clear();
      this.exposedSessionId = sessionId;
    }
    if (this.exposedFlags.has(key)) return;
    this.exposedFlags.add(key);
    this.track('$feature_flag', { flag_key: key, variant }, { eventCategory: 'experiment' });
  }

  private debugLog(message: string): void {
    if (!this.config.debug) return;
    (globalThis as { console?: { log?: (...a: unknown[]) => void } }).console?.log?.(
      '[uba-wechat]',
      message,
    );
  }
}
