// config.ets —— 配置与契约模型。
// 契约铁律（与各端 SDK 一致）：tenantId/serverTime/ip 服务端补全不发送；
// 数值标量（durationMs/quantity/score/sessionSeq/serverId/level）只进 behavior oneof。

export const SDK_VERSION: string = '1.0.0'
export const MAX_BATCH_BYTES: number = 900 * 1024

export class UbaConfig {
  appId: string = ''
  appSecret: string = ''
  endpoint: string = ''
  path: string = '/uba/v1/report'
  flagsPath: string = '/uba/v1/flags'
  batchSize: number = 20
  flushIntervalMs: number = 5000
  maxRetries: number = 3
  timeoutMs: number = 8000
  retryBaseDelayMs: number = 1000
  debug: boolean = false
  deviceId: string = '' // 空则由 context 层生成持久化 uuid（对齐移动端语义）
  platform: string = 'harmonyos'
  appVersion: string = ''
  channel: string = ''
  autoTrackLaunch: boolean = false
  pageViewEvent: string = 'page_view'
  sessionTimeoutMs: number = 30000
  sampleRate: number = 1.0
  requireConsent: boolean = false
  maxBatchBytes: number = MAX_BATCH_BYTES
  maxPersistedEvents: number = 500

  validate(): void {
    if (this.appId === '') {
      throw new Error('uba-harmonyos: appId 不能为空')
    }
    if (this.appSecret === '') {
      throw new Error('uba-harmonyos: appSecret 不能为空')
    }
    if (this.endpoint === '') {
      throw new Error('uba-harmonyos: endpoint 不能为空')
    }
    if (this.timeoutMs <= 0 || this.timeoutMs >= 10000) {
      throw new Error('uba-harmonyos: timeoutMs 必须 > 0 且 < 10000（服务端超时 10s）')
    }
    if (this.maxBatchBytes <= 0 || this.maxBatchBytes >= 1024 * 1024) {
      throw new Error('uba-harmonyos: maxBatchBytes 必须压在 1MiB 以内')
    }
    if (this.batchSize <= 0) {
      throw new Error('uba-harmonyos: batchSize 必须 > 0')
    }
    if (this.sampleRate < 0 || this.sampleRate > 1) {
      throw new Error('uba-harmonyos: sampleRate 必须在 0~1')
    }
  }

  getReportUrl(): string {
    return this.trimSlash(this.endpoint) + this.path
  }

  getFlagsUrl(): string {
    return this.trimSlash(this.endpoint) + this.flagsPath
  }

  private trimSlash(s: string): string {
    let out = s
    while (out.length > 0 && out[out.length - 1] === '/') {
      out = out.substring(0, out.length - 1)
    }
    return out
  }
}

/** 单次上报维度（蛇形命名对齐其他端 TrackOptions 字段语义）。 */
export class TrackOptions {
  eventCategory: string | null = null
  userId: number | null = null
  deviceId: string | null = null
  sessionId: string | null = null
  platform: string | null = null
  eventAction: string | null = null
  objectType: string | null = null
  objectId: string | null = null
  objectName: string | null = null
  durationMs: number | null = null
  amount: string | null = null
  quantity: number | null = null
  score: number | null = null
  metrics: Map<string, number> | null = null
  network: string | null = null
  opResult: string | null = null
  errorCode: string | null = null
  properties: Map<string, string> | null = null
  serverId: string | null = null
  level: number | null = null
  traceId: string | null = null
}

/** 风险事件载荷（risk oneof）。riskEventId/occurTime/reportTime 服务端补全。 */
export class RiskEvent {
  riskType: string | null = null
  riskLevel: string | null = null // HIGH / MEDIUM / LOW
  riskScore: number | null = null // 0-100
  ruleId: number | null = null
  ruleName: string | null = null
  ruleContextJson: string | null = null // 任意 JSON 对象的字符串表示（proto Struct）
  relatedEventIds: Array<string> | null = null
  description: string | null = null
  evidence: Map<string, string> | null = null
  status: string | null = null
}

/** 一次 flush 的结果。 */
export class FlushResult {
  success: boolean
  sent: number
  dropped: number
  retryable: boolean
  message: string | null

  constructor(success: boolean, sent: number, dropped: number,
              retryable: boolean = false, message: string | null = null) {
    this.success = success
    this.sent = sent
    this.dropped = dropped
    this.retryable = retryable
    this.message = message
  }
}
