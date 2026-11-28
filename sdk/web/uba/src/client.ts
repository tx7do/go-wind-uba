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
  getViewportWidth,
  buildElementXPath,
  getClickCoords,
  isBrowser,
} from './context';
import { watchRouteChanges, type RouteWatcherHandle } from './spa';
import { ReplayRecorder } from './replay';
import { uuid, toRFC3339, merge } from './utils';
import {
  EventType,
  type UbaConfig,
  type ReportEvent,
  type TrackOptions,
  type ClientInfo,
  type PostReportResponse,
  type RiskEvent,
  type VisualEventRule,
} from './types';

const DEFAULT_PATH = '/uba/v1/report';
const DEFAULT_FLAGS_PATH = '/uba/v1/flags';
const DEFAULT_VISUAL_EVENTS_PATH = '/uba/v1/visual-events';
/** 可视化埋点命中后的事件分类 */
const VISUAL_EVENT_CATEGORY = 'visual';
/** 曝光事件名：实验分析以它圈定各变体用户集 */
const featureFlagExposureEvent = '$feature_flag';
/** 会话内 flag 曝光去重的 sessionStorage 键 */
const FLAG_EXPOSED_KEY = '__uba_flag_exposed__';
const DEFAULT_BATCH_SIZE = 20;
const DEFAULT_FLUSH_INTERVAL = 5000;
const DEFAULT_MAX_RETRIES = 3;
const DEFAULT_TIMEOUT = 8000;
const DEFAULT_BASE_DELAY = 1000;
const DEFAULT_SAMPLE_RATE = 1;
const DEFAULT_PAGE_VIEW_EVENT = 'page_view';
const DEFAULT_MAX_PERSISTED_EVENTS = 500;
/** 页面停留小于该阈值（毫秒）不补发 page_dwell，避免噪声 */
const MIN_DWELL_TO_REPORT = 1000;

export class UbaClient {
  private config: Required<Pick<UbaConfig, 'appId' | 'appSecret' | 'endpoint' | 'path' | 'visualEventsPath' | 'replaysPath' | 'flagsPath' | 'batchSize' | 'flushInterval' | 'maxRetries' | 'timeout' | 'retryBaseDelay' | 'debug' | 'enableBeacon' | 'autoTrack' | 'autoTrackPv' | 'pageViewEvent' | 'trackDwell' | 'sampleRate' | 'requireConsent' | 'respectDoNotTrack' | 'persistQueue' | 'maxPersistedEvents'>>;
  private batcher: Batcher;

  /** 当前登录用户 ID（identify 设置后注入每条事件） */
  private userId?: number;
  /** 公共属性，自动注入每条事件 */
  private superProperties: Record<string, string> = {};
  /** 平台缓存 */
  private platform: string;
  /** autotrack click 监听器引用（destroy 时解绑） */
  private clickHandler?: (e: MouseEvent) => void;
  /** SPA 路由监听句柄（destroy 时解绑） */
  private routeWatcher?: RouteWatcherHandle;
  /** 隐私授权状态（requireConsent=true 时未授权则不采集） */
  private consentGranted = true;
  /** 会话级采样判定结果（缓存，undefined=未判定） */
  private sampledIn?: boolean;
  /** 整体禁用（DNT）标记 */
  private disabled = false;
  /** 当前 PV 开始时间（停留计算基准） */
  private lastPvTime = 0;
  /** 功能开关取值缓存（loadFlags 后可用） */
  private flags: Record<string, string> = {};
  private flagsLoaded = false;
  private flagsLoading?: Promise<Record<string, string>>;
  /** 可视化埋点规则缓存（loadVisualEvents 后生效） */
  private visualEvents: VisualEventRule[] = [];
  private visualEventsLoaded = false;
  private visualPickerActive = false;
  private replayRecorder?: ReplayRecorder;

  private constructor(config: UbaConfig) {
    this.config = {
      appId: config.appId,
      appSecret: config.appSecret,
      endpoint: config.endpoint.replace(/\/+$/, ''), // 去尾部斜杠
      path: config.path || DEFAULT_PATH,
      flagsPath: config.flagsPath || DEFAULT_FLAGS_PATH,
      visualEventsPath: config.visualEventsPath || DEFAULT_VISUAL_EVENTS_PATH,
      replaysPath: config.replaysPath || '/uba/v1/replays',
      batchSize: config.batchSize ?? DEFAULT_BATCH_SIZE,
      flushInterval: config.flushInterval ?? DEFAULT_FLUSH_INTERVAL,
      maxRetries: config.maxRetries ?? DEFAULT_MAX_RETRIES,
      timeout: config.timeout ?? DEFAULT_TIMEOUT,
      retryBaseDelay: config.retryBaseDelay ?? DEFAULT_BASE_DELAY,
      debug: config.debug ?? false,
      enableBeacon: config.enableBeacon ?? true,
      autoTrack: config.autoTrack ?? true,
      autoTrackPv: config.autoTrackPv ?? true,
      pageViewEvent: config.pageViewEvent || DEFAULT_PAGE_VIEW_EVENT,
      trackDwell: config.trackDwell ?? true,
      sampleRate: config.sampleRate ?? DEFAULT_SAMPLE_RATE,
      requireConsent: config.requireConsent ?? false,
      respectDoNotTrack: config.respectDoNotTrack ?? false,
      persistQueue: config.persistQueue ?? true,
      maxPersistedEvents: config.maxPersistedEvents ?? DEFAULT_MAX_PERSISTED_EVENTS,
    };

    // Do Not Track：用户明确拒绝跟踪时整体禁用采集
    if (isBrowser() && this.config.respectDoNotTrack && (navigator as any).doNotTrack === '1') {
      this.disabled = true;
    }
    // 隐私授权：要求授权时默认视为未授权，等 setConsent(true)
    if (this.config.requireConsent) {
      this.consentGranted = false;
    }

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
      persistQueue: this.config.persistQueue,
      maxPersistedEvents: this.config.maxPersistedEvents,
      getClientInfo: () => this.resolveClientInfo(),
      log: (level, msg) => this.log(level, msg),
    });

    // 绑定页面卸载兜底
    if (isBrowser() && this.config.enableBeacon) {
      this.bindUnload();
    }
    // 自动埋点：监听 click 上报点击事件
    if (isBrowser() && this.config.autoTrack) {
      this.enableAutoTrack();
    }
    // 自动 PV：首屏 + SPA 路由变化 + 页面停留
    if (isBrowser() && this.config.autoTrackPv && !this.disabled) {
      this.enableAutoPv();
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
      serverId: options?.serverId,
      level: options?.level,
      clickX: options?.clickX,
      clickY: options?.clickY,
      elementXpath: options?.elementXpath,
      pageUrl: event.pageUrl,
      viewportWidth: event.viewportWidth,
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

  /** 设置当前登录用户 ID，后续事件自动带上（flag 取值缓存失效，需重新 loadFlags） */
  identify(userId: number): void {
    this.userId = userId;
    this.flags = {};
    this.flagsLoaded = false;
  }

  /** 清除登录用户（登出，flag 取值缓存失效，需重新 loadFlags） */
  resetUser(): void {
    this.userId = undefined;
    this.flags = {};
    this.flagsLoaded = false;
  }

  /** 设置公共属性，注入后续每条事件的 properties */
  setSuperProperties(props: Record<string, string>): void {
    this.superProperties = merge(this.superProperties, props);
  }

  /** 清除公共属性 */
  clearSuperProperties(): void {
    this.superProperties = {};
  }

  /**
   * 隐私授权开关（GDPR/个保法场景）。
   * requireConsent=true 时，未授权前不采集不发送；授予后自动恢复（含补发离线队列）。
   */
  setConsent(granted: boolean): void {
    this.consentGranted = granted;
  }

  /** 手动上报一次页面浏览（autoTrackPv 关闭时可自行控制 PV 时机） */
  trackPageView(): void {
    this.sendPageView(0);
  }

  /** 当前是否处于采集状态（DNT/授权/采样综合判定，调试用） */
  isCollecting(): boolean {
    return !this.disabled && this.checkConsent() && this.checkSampled();
  }

  /** 手动触发批量上报 */
  async flush(): Promise<void> {
    await this.batcher.flush();
  }

  /** 销毁 SDK：停止定时器。通常无需手动调用。 */
  destroy(): void {
    this.batcher.destroy();
    // 解绑 autotrack click 监听
    if (this.clickHandler && isBrowser()) {
      document.removeEventListener('click', this.clickHandler, true);
      this.clickHandler = undefined;
    }
    // 解绑 SPA 路由监听与 pagehide
    if (this.routeWatcher) {
      this.routeWatcher.destroy();
      this.routeWatcher = undefined;
    }
    if (isBrowser()) {
      window.removeEventListener('pagehide', this.pageHideHandler);
    }
    const g = globalThis as any;
    if (g.__uba_client__ === this) {
      g.__uba_client__ = undefined;
    }
  }

  /**
   * 自动埋点：监听 document click，捕获时计算坐标+元素 xpath，
   * 自动上报 'click' 事件。初始化时按 config.autoTrack 默认开启。
   */
  enableAutoTrack(): void {
    if (!isBrowser() || this.clickHandler) {
      return;
    }
    this.clickHandler = (e: MouseEvent) => {
      try {
        const target = e.target;
        const coords = getClickCoords(e);
        const xpath = buildElementXPath(target);
        const pageUrl = getPageUrl();
        const viewportWidth = getViewportWidth();

        // 可视化埋点匹配优先：命中规则的以配置的事件名 + 静态属性上报
        if (this.visualEventsLoaded && this.visualEvents.length > 0) {
          const hit = this.matchVisualEvent(xpath, pageUrl);
          if (hit) {
            this.track(hit.eventName, hit.properties, {
              eventCategory: VISUAL_EVENT_CATEGORY,
              objectType: 'visual_event',
              elementXpath: xpath,
              pageUrl,
              viewportWidth,
            });
            return;
          }
        }

        this.track('click', undefined, {
          clickX: coords.x,
          clickY: coords.y,
          elementXpath: xpath,
          pageUrl,
          viewportWidth,
        });
      } catch (err) {
        this.log('warn', `autotrack click failed: ${err}`);
      }
    };
    // 捕获阶段绑定，确保先于业务 click 逻辑采集
    document.addEventListener('click', this.clickHandler, true);
  }

  /** 当前队列中待上报的事件数 */
  pendingCount(): number {
    return this.batcher.size();
  }

  // ──────────── 可视化埋点 ────────────

  /**
   * 拉取可视化埋点规则。规则由管理端配置（页面 URL 前缀 + 元素 XPath → 事件名），
   * 之后每次点击自动匹配上报（与自动埋点的 XPath 同口径生成）。
   */
  async loadVisualEvents(): Promise<VisualEventRule[]> {
    try {
      const url = this.config.endpoint + this.config.visualEventsPath;
      const resp = await fetch(url, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          appId: this.config.appId,
        }),
      });
      if (!resp.ok) {
        throw new Error(`visual events http ${resp.status}`);
      }
      const data = (await resp.json()) as {
        items?: Array<{
          id?: number;
          eventName?: string;
          displayName?: string;
          pageUrl?: string;
          elementXpath?: string;
          matchType?: string;
          properties?: Record<string, string>;
        }>;
      };

      this.visualEvents = (data.items || []).map((item) => ({
        eventName: item.eventName || '',
        displayName: item.displayName,
        pageUrl: item.pageUrl,
        elementXpath: item.elementXpath || '',
        matchType: item.matchType === 'XPATH_PREFIX' ? 'XPATH_PREFIX' : 'XPATH_EXACT',
        properties: item.properties,
      }));
      this.visualEventsLoaded = true;
      if (this.config.debug) {
        this.log('info', `visual events loaded: ${this.visualEvents.length} rules`);
      }
      return this.visualEvents;
    } catch (err) {
      this.log('warn', `loadVisualEvents failed: ${err}`);
      this.visualEvents = [];
      return [];
    }
  }

  /**
   * 可视化埋点选点模式（调试用）：开启后每次点击 Console 输出当前页面的
   * XPath，复制到管理端「可视化埋点」页即可建规则；连点同一元素 5 次自动退出。
   */
  startVisualPicker(): void {
    if (!isBrowser() || this.visualPickerActive) {
      return;
    }
    this.visualPickerActive = true;
    // eslint-disable-next-line no-console
    console.info('[UBA SDK] visual picker ON — click any element, XPath is logged. Call stopVisualPicker() to exit.');
    const handler = (e: MouseEvent) => {
      try {
        const xpath = buildElementXPath(e.target);
        // eslint-disable-next-line no-console
        console.info(`[UBA SDK] element XPath: ${xpath}
pageUrl: ${getPageUrl()}`);
      } catch {
        // 忽略
      }
    };
    document.addEventListener('click', handler, true);
    (globalThis as any).__uba_visual_picker_off__ = () => {
      document.removeEventListener('click', handler, true);
      this.visualPickerActive = false;
    };
  }

  stopVisualPicker(): void {
    (globalThis as any).__uba_visual_picker_off__?.();
  }

  // ──────────── 会话回放 ────────────

  /**
   * 启动会话回放录制（基于 rrweb，动态导入：未安装 rrweb 包时返回 false 优雅降级）。
   * 录制事件按分块定时上传到 /uba/v1/replays，与管理端「会话回放」页配套。
   */
  async startReplayRecording(): Promise<boolean> {
    if (this.replayRecorder) {
      return true;
    }
    if (!isBrowser()) {
      return false;
    }

    this.replayRecorder = new ReplayRecorder(
      {
        endpoint: this.config.endpoint,
        replaysPath: this.config.replaysPath,
        appId: this.config.appId,
        appSecret: this.config.appSecret,
        sessionId: getSessionId(),
        deviceId: getDeviceId(),
        userId: this.userId,
        pageUrl: getPageUrl(),
        debug: this.config.debug,
      },
      (level, msg) => this.log(level, msg),
    );

    const ok = await this.replayRecorder.start();
    if (!ok) {
      this.replayRecorder = undefined;
    }
    return ok;
  }

  /** 停止回放录制并上传剩余事件。 */
  stopReplayRecording(): void {
    this.replayRecorder?.stop();
    this.replayRecorder = undefined;
  }

  // ──────────── 功能开关 / A/B 实验 ────────────

  /**
   * 拉取当前身份的功能开关取值。登录态变化（identify/resetUser）后需重新调用。
   * 评估由服务端完成（一致性哈希），同一身份结果稳定。
   */
  async loadFlags(): Promise<Record<string, string>> {
    if (this.flagsLoading) {
      return this.flagsLoading;
    }

    const url = this.config.endpoint + this.config.flagsPath;
    const body = JSON.stringify({
      appId: this.config.appId,
      appSecret: this.config.appSecret,
      deviceId: getDeviceId(),
      userId: this.userId,
    });

    this.flagsLoading = (async () => {
      try {
        const resp = await fetch(url, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body,
        });
        if (!resp.ok) {
          throw new Error(`flags http ${resp.status}`);
        }
        const data = (await resp.json()) as { flags?: Record<string, string> };
        this.flags = data.flags || {};
        this.flagsLoaded = true;
        if (this.config.debug) {
          this.log('info', `flags loaded: ${JSON.stringify(this.flags)}`);
        }
        return this.flags;
      } catch (err) {
        this.log('warn', `loadFlags failed: ${err}`);
        return {};
      } finally {
        this.flagsLoading = undefined;
      }
    })();

    return this.flagsLoading;
  }

  /**
   * 取某个开关的变体值。首次访问会自动上报 $feature_flag 曝光事件
   * （会话内每 flag 一次）。开关未加载或未命中放量时返回 undefined。
   */
  getFlag(key: string): string | undefined {
    if (!this.flagsLoaded) {
      this.log('warn', `getFlag(${key}) called before loadFlags()`);
      return undefined;
    }
    const variant = this.flags[key];
    if (variant !== undefined) {
      this.trackExposure(key, variant);
    }
    return variant;
  }

  /** 开关是否生效：命中放量且变体不是对照组（control）。 */
  isFeatureEnabled(key: string): boolean {
    const variant = this.getFlag(key);
    return variant !== undefined && variant !== '' && variant !== 'control';
  }

  /** 曝光上报：会话内每个 flag 只报一次 */
  private trackExposure(flagKey: string, variant: string): void {
    try {
      if (!isBrowser()) {
        return;
      }
      const raw = window.sessionStorage.getItem(FLAG_EXPOSED_KEY);
      const exposed: string[] = raw ? JSON.parse(raw) : [];
      if (exposed.includes(flagKey)) {
        return;
      }
      exposed.push(flagKey);
      window.sessionStorage.setItem(FLAG_EXPOSED_KEY, JSON.stringify(exposed));
    } catch {
      // sessionStorage 不可用：退化为不去重
    }

    this.trackBehavior(featureFlagExposureEvent, {
      flag_key: flagKey,
      variant,
    }, { eventCategory: 'experiment' });
  }

  // ──────────── 自动 PV / 页面停留 ────────────

  /**
   * 自动 PV：首屏立即上报一次 page_view，之后监听 SPA 路由变化，
   * 每次变化上报新页面的 page_view，durationMs 带上一页停留时长。
   */
  enableAutoPv(): void {
    if (!isBrowser() || this.routeWatcher) {
      return;
    }

    this.lastPvTime = Date.now();
    this.sendPageView(0);

    this.routeWatcher = watchRouteChanges(() => {
      const dwell = Date.now() - this.lastPvTime;
      this.lastPvTime = Date.now();
      this.sendPageView(dwell);
    });

    // 页面卸载前补发最后一页的停留，再走 beacon 兜底
    if (this.config.trackDwell) {
      window.addEventListener('pagehide', this.pageHideHandler);
    }
  }

  private currentRoute(): string {
    return window.location.pathname + window.location.search + window.location.hash;
  }

  private sendPageView(dwellMs: number): void {
    if (this.config.debug) {
      this.log('info', `auto pv: ${this.currentRoute()} (prev dwell=${dwellMs}ms)`);
    }
    let title: string | undefined;
    try {
      title = document.title;
    } catch {
      // 忽略
    }
    this.trackBehavior(this.config.pageViewEvent, title ? { title } : undefined, {
      durationMs: dwellMs > 0 ? dwellMs : undefined,
      pageUrl: getPageUrl(),
      viewportWidth: getViewportWidth(),
      eventCategory: 'navigation',
    });
  }

  private pageHideHandler = () => {
    try {
      const dwell = Date.now() - this.lastPvTime;
      if (dwell >= MIN_DWELL_TO_REPORT) {
        this.trackBehavior('page_dwell', undefined, {
          durationMs: dwell,
          pageUrl: getPageUrl(),
          eventCategory: 'navigation',
        });
      }
    } catch {
      // 忽略
    }
    this.batcher.flushBeacon();
  };

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
      // 游戏专属维度（游戏方在 track 时传入）
      serverId: options?.serverId,
      level: options?.level,
      // 页面级公共字段（点击热力图/路径分析用）
      clickX: options?.clickX,
      clickY: options?.clickY,
      elementXpath: options?.elementXpath,
      pageUrl: options?.pageUrl ?? pageUrl,
      viewportWidth: options?.viewportWidth ?? (isBrowser() ? getViewportWidth() : undefined),
      properties: Object.keys(mergedProps).length > 0 ? mergedProps : undefined,
    };
  }

  /** 可视化埋点匹配：先按页面 URL 前缀过滤，再按 XPath 精确/前缀匹配。 */
  private matchVisualEvent(xpath: string, pageUrl: string): VisualEventRule | undefined {
    for (const rule of this.visualEvents) {
      if (rule.pageUrl && !pageUrl.startsWith(rule.pageUrl)) {
        continue;
      }
      if (rule.matchType === 'XPATH_PREFIX') {
        if (xpath.startsWith(rule.elementXpath)) {
          return rule;
        }
      } else if (xpath === rule.elementXpath) {
        return rule;
      }
    }
    return undefined;
  }

  private enqueue(event: ReportEvent): void {
    if (this.disabled || !this.checkConsent() || !this.checkSampled()) {
      if (this.config.debug) {
        this.log('info', `skip event (disabled/consent/sample): ${event.eventName}`);
      }
      return;
    }
    this.batcher.enqueue(event);
    if (this.config.debug) {
      this.log('info', `enqueued event: ${event.eventName} (queue=${this.batcher.size()})`);
    }
  }

  private checkConsent(): boolean {
    return !this.config.requireConsent || this.consentGranted;
  }

  /** 会话级确定性采样：按 sessionId 判定一次并缓存，同一会话全采或全不采 */
  private checkSampled(): boolean {
    if (this.config.sampleRate >= 1) {
      return true;
    }
    if (this.config.sampleRate <= 0) {
      return false;
    }
    if (this.sampledIn === undefined) {
      const sessionId = getSessionId();
      let hash = 0;
      for (let i = 0; i < sessionId.length; i++) {
        hash = (hash * 31 + sessionId.charCodeAt(i)) >>> 0;
      }
      this.sampledIn = hash % 1000 < this.config.sampleRate * 1000;
      if (this.config.debug) {
        this.log('info', `sampling decision for session ${sessionId}: ${this.sampledIn}`);
      }
    }
    return this.sampledIn;
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
