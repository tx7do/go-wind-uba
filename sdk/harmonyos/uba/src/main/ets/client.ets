// client.ets —— GoWind UBA 鸿蒙埋点客户端（线程安全：单线程事件循环 + 异步 flush）。
// 契约铁律（与各端 SDK 一致）：camelCase 对齐 proto json_name；
// tenantId/serverTime/ip 服务端补全不发送；数值标量只进 behavior oneof；
// 4xx（除 429）不重试；网络/5xx/429 耗尽重试后回队补发。
import { FlushResult, RiskEvent, TrackOptions, UbaConfig } from './config'
import { nowMs, toRFC3339Ms, uuidV4 } from './utils'
import { SessionManager } from './session'
import { HttpResult, postJson } from './transport'

// wire 事件（已序列化的 JSON 字符串，失败回队补发无损）
interface Queue {
  items: Array<string>
}

function putField(obj: Record<string, Object | null>, key: string, value: Object | null): void {
  if (value === null) {
    return
  }
  obj[key] = value
}

export class UbaClient {
  private readonly config: UbaConfig
  private readonly session: SessionManager
  private queue: Array<string> = []
  private restored: Array<string> = [] // 离盘恢复（预留：preferences 持久化，见 AGENTS 说明）
  private flushing: boolean = false
  private closed: boolean = false
  private userId: number | null = null
  private superProps: Map<string, string> = new Map()
  private flags: Map<string, string> = new Map()
  private exposedFlags: Set<string> = new Set<string>()
  private exposedSession: string | null = null
  private lastScreenAt: number | null = null
  private consentGranted: boolean = false
  private readonly deviceId: string

  private constructor(config: UbaConfig, deviceId: string) {
    this.config = config
    this.deviceId = deviceId
    this.session = new SessionManager(config.sessionTimeoutMs)
  }

  /** 初始化（异步：device id 恢复来自 preferences）。在 EntryAbility onCreate 调用。 */
  static async create(config: UbaConfig, deviceId: string): Promise<UbaClient> {
    config.validate()
    return new UbaClient(config, deviceId)
  }

  // ─── 上报 API ────────────────────────────────────────────────────────────

  /** 登录绑定用户 ID。 */
  identify(userId: number): void {
    this.userId = userId
  }

  /** 登出解绑。 */
  resetUser(): void {
    this.userId = null
  }

  /** 设置全局公共属性：合并进之后每个事件（同名覆盖）。 */
  setSuperProperties(props: Map<string, string>): void {
    props.forEach((value: string, key: string) => {
      this.superProps.set(key, value)
    })
  }

  /** 清空全局公共属性。 */
  clearSuperProperties(): void {
    this.superProps.clear()
  }

  /** 上报自定义行为事件。可重试失败留在内存队列等待补发。 */
  track(eventName: string, properties: Map<string, string> | null = null,
        options: TrackOptions | null = null): void {
    this.trackBehavior(eventName, properties, options)
  }

  trackBehavior(eventName: string, properties: Map<string, string> | null = null,
                options: TrackOptions | null = null): void {
    const wire = this.buildBehaviorWire(eventName, properties, options)
    if (wire === null) {
      return
    }
    this.enqueue(wire)
  }

  /** 上报风险事件（进入 uba_risk_events 通道）。 */
  trackRisk(eventName: string, risk: RiskEvent,
            properties: Map<string, string> | null = null,
            options: TrackOptions | null = null): void {
    if (!this.shouldCollect(eventName)) {
      return
    }
    const opts = options === null ? new TrackOptions() : options
    const top: Record<string, Object | null> = this.commonWire(
      'RISK', eventName, uuidV4(), toRFC3339Ms(nowMs()), opts,
      this.mergedProperties(properties, opts))
    const riskWire: Record<string, Object | null> = {}
    putField(riskWire, 'riskType', risk.riskType)
    putField(riskWire, 'riskLevel', risk.riskLevel)
    putField(riskWire, 'riskScore', risk.riskScore)
    putField(riskWire, 'ruleId', risk.ruleId)
    putField(riskWire, 'ruleName', risk.ruleName)
    if (risk.ruleContextJson !== null) {
      putField(riskWire, 'ruleContext', JSON.parse(risk.ruleContextJson) as Object)
    }
    if (risk.relatedEventIds !== null && risk.relatedEventIds.length > 0) {
      putField(riskWire, 'relatedEventIds', risk.relatedEventIds)
    }
    putField(riskWire, 'description', risk.description)
    if (risk.evidence !== null && risk.evidence.size > 0) {
      const ev: Record<string, string> = {}
      risk.evidence.forEach((value: string, key: string) => {
        ev[key] = value
      })
      putField(riskWire, 'evidence', ev)
    }
    putField(riskWire, 'status', risk.status)
    putField(top, 'risk', riskWire)
    this.enqueue(JSON.stringify(top))
  }

  /** 页面浏览事件：自动携带上一页面停留时长 durationMs。 */
  trackPageView(screenName: string, title: string | null = null,
                referer: string | null = null): void {
    const now = nowMs()
    const dwell = this.lastScreenAt === null ? null : now - this.lastScreenAt
    this.lastScreenAt = now
    const props: Map<string, string> = new Map()
    props.set('pageUrl', screenName)
    if (title !== null && title !== '') {
      props.set('title', title)
    }
    if (referer !== null && referer !== '') {
      props.set('referer', referer)
    }
    const opts = new TrackOptions()
    opts.eventCategory = 'navigation'
    opts.objectType = 'page'
    opts.objectId = screenName
    if (dwell !== null && dwell > 0) {
      opts.durationMs = dwell
    }
    this.trackBehavior(this.config.pageViewEvent, props, opts)
  }

  /** 隐私合规授权开关；requireConsent = true 时未授权前所有上报静默丢弃。 */
  setConsent(granted: boolean): void {
    this.consentGranted = granted
  }

  /** 当前是否处于采集状态（未关闭、consent 放行、会话命中采样）。 */
  isCollecting(): boolean {
    return !this.closed && this.consentOk() && this.session.inSample(this.config.sampleRate)
  }

  // ─── 生命周期（Ability onForeground/onBackground 接入） ───────────────────

  /** 退到后台：立即 flush + 记录时刻。 */
  handleBackground(): Promise<FlushResult> {
    this.session.onBackground()
    return this.flush()
  }

  /** 回到前台：后台超过 sessionTimeoutMs 则轮换新会话。 */
  handleForeground(): void {
    this.session.onForeground()
  }

  // ─── 维护 ────────────────────────────────────────────────────────────────

  /** 队列中待上报条数。 */
  pendingCount(): number {
    return this.queue.length + this.restored.length
  }

  /** 手动冲刷队列：分批发送直到清空或遇到失败。并发调用安全（重入返回成功）。 */
  async flush(): Promise<FlushResult> {
    if (this.closed) {
      return new FlushResult(true)
    }
    if (this.flushing) {
      return new FlushResult(true)
    }
    this.flushing = true
    let sent = 0
    let dropped = 0
    let ok = true
    let retryable = false
    let message: string | null = null
    try {
      // 1. 补发恢复的离线事件
      while (this.restored.length > 0) {
        const batch = this.takeRestoredBatch()
        if (batch.length === 0) {
          break
        }
        const r = await this.sendWithRetry(batch)
        sent += r.sent
        dropped += r.dropped
        if (!r.success) {
          ok = false
          retryable = r.retryable
          message = r.message
          if (retryable) {
            this.restored = batch.concat(this.restored)
          }
          break
        }
      }
      // 2. 发送内存队列新事件
      if (ok) {
        while (this.queue.length > 0) {
          const batch = this.takeBatch()
          if (batch.length === 0) {
            break
          }
          const r = await this.sendWithRetry(batch)
          sent += r.sent
          dropped += r.dropped
          if (!r.success) {
            ok = false
            retryable = r.retryable
            message = r.message
            if (retryable) {
              this.queue = batch.concat(this.queue)
            }
            break
          }
        }
      }
    } finally {
      this.flushing = false
    }
    return new FlushResult(ok, sent, dropped, retryable, message)
  }

  /** 关闭：冲刷剩余队列并拒绝后续入队。 */
  async close(): Promise<FlushResult> {
    const result = await this.flush()
    this.closed = true
    return result
  }

  // ─── AB 实验开关 ──────────────────────────────────────────────────────────

  /** 拉取最新实验开关并缓存。失败保留旧缓存。 */
  async loadFlags(): Promise<Map<string, string>> {
    const flagsBody: Record<string, Object | null> = {
      'appId': this.config.appId,
      'appSecret': this.config.appSecret,
      'deviceId': this.deviceId
    }
    if (this.userId !== null) {
      flagsBody['userId'] = this.userId
    }
    const res = await postJson(this.config.getFlagsUrl(), JSON.stringify(flagsBody),
      this.config.timeoutMs)
    if (res.isOk()) {
      const parsed = JSON.parse(res.body) as Record<string, Object>
      const raw = parsed['flags']
      const next: Map<string, string> = new Map()
      if (raw !== undefined && raw !== null) {
        const record = raw as Record<string, string>
        Object.keys(record).forEach((key: string) => {
          next.set(key, record[key])
        })
      }
      this.flags = next
    } else {
      this.debugLog(`拉取 flags 失败：HTTP ${res.statusCode}`)
    }
    const out: Map<string, string> = new Map()
    this.flags.forEach((value: string, key: string) => {
      out.set(key, value)
    })
    return out
  }

  /** 读取开关变体；命中时自动上报一次 $feature_flag 曝光（每会话每开关一次）。 */
  getFlag(key: string): string | null {
    const variant = this.flags.get(key)
    if (variant === undefined) {
      return null
    }
    this.trackFlagExposure(key, variant)
    return variant
  }

  /** 变体非空且非 control 即视为命中（不触发曝光事件）。 */
  isFeatureEnabled(key: string): boolean {
    const v = this.flags.get(key)
    return v !== undefined && v !== '' && v !== 'control'
  }

  // ─── 内部实现 ────────────────────────────────────────────────────────────

  private consentOk(): boolean {
    return !this.config.requireConsent || this.consentGranted
  }

  private shouldCollect(eventName: string): boolean {
    if (this.closed) {
      return false
    }
    if (!this.consentOk()) {
      this.debugLog(`consent 未授权，丢弃事件 ${eventName}`)
      return false
    }
    return this.session.inSample(this.config.sampleRate)
  }

  private commonWire(eventType: string, eventName: string, eventId: string,
                     eventTime: string, opts: TrackOptions,
                     properties: Map<string, string> | null): Record<string, Object | null> {
    const m: Record<string, Object | null> = {
      'eventType': eventType,
      'eventId': eventId,
      'eventName': eventName,
      'eventTime': eventTime
    }
    const uid = opts.userId !== null ? opts.userId : this.userId
    if (uid !== null) {
      m['userId'] = uid
    }
    m['deviceId'] = opts.deviceId !== null ? opts.deviceId : this.deviceId
    m['sessionId'] = opts.sessionId !== null ? opts.sessionId : this.session.current
    m['platform'] = opts.platform !== null ? opts.platform : this.config.platform
    putField(m, 'eventCategory', opts.eventCategory)
    if (properties !== null && properties.size > 0) {
      const p: Record<string, string> = {}
      properties.forEach((value: string, key: string) => {
        p[key] = value
      })
      m['properties'] = p
    }
    if (opts.metrics !== null && opts.metrics.size > 0) {
      const mm: Record<string, number> = {}
      opts.metrics.forEach((value: number, key: string) => {
        mm[key] = value
      })
      m['metrics'] = mm
    }
    putField(m, 'traceId', opts.traceId)
    // 顶层字符串镜像维度；数值标量只进 behavior oneof
    putField(m, 'eventAction', opts.eventAction)
    putField(m, 'objectType', opts.objectType)
    putField(m, 'objectId', opts.objectId)
    putField(m, 'objectName', opts.objectName)
    putField(m, 'amount', opts.amount)
    putField(m, 'opResult', opts.opResult)
    putField(m, 'errorCode', opts.errorCode)
    return m
  }

  private buildBehaviorWire(eventName: string, properties: Map<string, string> | null,
                            options: TrackOptions | null): string | null {
    if (!this.shouldCollect(eventName)) {
      return null
    }
    if (options === null || options.sessionId === null) {
      if (this.config.sessionTimeoutMs > 0) {
        this.session.checkTimeout()
      }
    }
    const opts = options === null ? new TrackOptions() : options
    const top = this.commonWire('BEHAVIOR', eventName, uuidV4(), toRFC3339Ms(nowMs()),
      opts, this.mergedProperties(properties, opts))
    // behavior oneof 必须存在；数值标量只在这里才被服务端信任
    const b: Record<string, Object | null> = { 'os': 'HarmonyOS' }
    if (this.config.appVersion !== '') {
      b['appVersion'] = this.config.appVersion
    }
    if (this.config.channel !== '') {
      b['channel'] = this.config.channel
    }
    putField(b, 'eventAction', opts.eventAction)
    putField(b, 'objectType', opts.objectType)
    putField(b, 'objectId', opts.objectId)
    putField(b, 'objectName', opts.objectName)
    putField(b, 'network', opts.network)
    putField(b, 'durationMs', opts.durationMs)
    putField(b, 'amount', opts.amount)
    putField(b, 'quantity', opts.quantity)
    putField(b, 'score', opts.score)
    if (opts.metrics !== null && opts.metrics.size > 0) {
      const mm: Record<string, number> = {}
      opts.metrics.forEach((value: number, key: string) => {
        mm[key] = value
      })
      b['metrics'] = mm
    }
    putField(b, 'opResult', opts.opResult)
    putField(b, 'errorCode', opts.errorCode)
    putField(b, 'serverId', opts.serverId)
    putField(b, 'level', opts.level)
    top['behavior'] = b
    return JSON.stringify(top)
  }

  private mergedProperties(properties: Map<string, string> | null,
                           options: TrackOptions | null): Map<string, string> | null {
    const merged: Map<string, string> = new Map()
    this.superProps.forEach((value: string, key: string) => {
      merged.set(key, value)
    })
    if (properties !== null) {
      properties.forEach((value: string, key: string) => {
        merged.set(key, value)
      })
    }
    if (options !== null && options.properties !== null) {
      options.properties.forEach((value: string, key: string) => {
        merged.set(key, value)
      })
    }
    return merged.size > 0 ? merged : null
  }

  private enqueue(wire: string): void {
    this.queue.push(wire)
    if (this.queue.length >= this.config.batchSize) {
      this.flushInBackground()
    }
  }

  private flushInBackground(): void {
    // 异步触发，不阻塞调用方
    this.flush().then(() => {}).catch(() => {})
  }

  private takeBatch(): Array<string> {
    const batch: Array<string> = []
    let bytes = 0
    while (this.queue.length > 0 && batch.length < this.config.batchSize) {
      const next = this.queue[0]
      if (batch.length > 0 && bytes + next.length > this.config.maxBatchBytes) {
        break
      }
      bytes += next.length
      batch.push(this.queue.shift() as string)
    }
    return batch
  }

  private takeRestoredBatch(): Array<string> {
    const batch: Array<string> = []
    let bytes = 0
    while (this.restored.length > 0 && batch.length < this.config.batchSize) {
      const next = this.restored[0]
      if (batch.length > 0 && bytes + next.length > this.config.maxBatchBytes) {
        break
      }
      bytes += next.length
      batch.push(this.restored.shift() as string)
    }
    return batch
  }

  private async sendWithRetry(batch: Array<string>): Promise<FlushResult> {
    const bodyObj: Record<string, Object | null> = {
      'appId': this.config.appId,
      'appSecret': this.config.appSecret,
      'clientInfo': { 'userAgent': `uba-harmonyos/1.0.0 (harmonyos)` },
      'events': batch
    }
    const body = JSON.stringify(bodyObj)
    for (let attempt = 0; ; attempt++) {
      const res = await postJson(this.config.getReportUrl(), body, this.config.timeoutMs)
      if (res.isOk()) {
        const failed = this.parseFailedCount(res.body)
        if (failed > 0) {
          this.debugLog(`部分事件被服务端拒绝：failedCount=${failed}`)
        }
        return new FlushResult(true, batch.length - failed, failed)
      }
      // 4xx（除 429）是确定性失败：整批丢弃，不重试
      if (res.statusCode >= 400 && res.statusCode < 500 && res.statusCode !== 429) {
        this.debugLog(`HTTP ${res.statusCode}（不可重试），丢弃 ${batch.length} 条`)
        return new FlushResult(false, 0, batch.length, false, `HTTP ${res.statusCode}`)
      }
      if (attempt >= this.config.maxRetries) {
        this.debugLog(`重试耗尽（HTTP ${res.statusCode}），${batch.length} 条待补发`)
        return new FlushResult(false, 0, batch.length, true,
          `HTTP ${res.statusCode} after ${attempt + 1} attempts`)
      }
      const delay = this.config.retryBaseDelayMs * Math.pow(2, Math.min(attempt, 30))
      this.debugLog(`HTTP ${res.statusCode}，${delay}ms 后第 ${attempt + 1} 次重试`)
      await new Promise<void>((resolve) => setTimeout(resolve, delay))
    }
  }

  private parseFailedCount(body: string): number {
    try {
      const parsed = JSON.parse(body) as Record<string, Object>
      const v = parsed['failedCount']
      if (typeof v === 'number') {
        return v
      }
    } catch (err) {
      // 非 JSON 响应按 0 处理
    }
    return 0
  }

  private trackFlagExposure(key: string, variant: string): void {
    const sessionId = this.session.current
    if (this.exposedSession !== sessionId) {
      this.exposedFlags.clear()
      this.exposedSession = sessionId
    }
    if (this.exposedFlags.has(key)) {
      return
    }
    this.exposedFlags.add(key)
    const props: Map<string, string> = new Map()
    props.set('flag_key', key)
    props.set('variant', variant)
    const opts = new TrackOptions()
    opts.eventCategory = 'experiment'
    this.track('$feature_flag', props, opts)
  }

  private debugLog(message: string): void {
    if (this.config.debug) {
      console.log(`[uba-harmonyos] ${message}`)
    }
  }
}
