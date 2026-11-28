/**
 * 上报契约类型，字段名与后端 proto（uba/service/v1/report.proto 等的 json_name）
 * 逐一对齐，序列化输出 camelCase 并跳过 undefined/null。
 *
 * 铁律：tenantId/serverTime/ip 服务端补全不发送；数值标量（durationMs/quantity/
 * score/sessionSeq/serverId/level）只进 behavior oneof 才被服务端信任。
 */

/** 输出对象里只保留非 null/undefined 字段（空对象/空数组也跳过）。 */
export function putIfPresent<T extends Record<string, unknown>>(
  m: T,
  key: string,
  value: unknown,
): void {
  if (value == null) return;
  if (typeof value === 'object' && !Array.isArray(value) && Object.keys(value).length === 0) return;
  if (Array.isArray(value) && value.length === 0) return;
  (m as Record<string, unknown>)[key] = value;
}

export type EventType = 'BEHAVIOR' | 'RISK';

export interface ClientInfo {
  userAgent?: string;
  referer?: string;
  country?: string;
  city?: string;
}

export interface BehaviorPayload {
  eventAction?: string;
  objectType?: string;
  objectId?: string;
  objectName?: string;
  sessionSeq?: number;
  os?: string;
  appVersion?: string;
  channel?: string;
  network?: string;
  durationMs?: number;
  amount?: string;
  quantity?: number;
  score?: number;
  metrics?: Record<string, number>;
  properties?: Record<string, string>;
  opResult?: string;
  errorCode?: string;
  clickX?: number;
  clickY?: number;
  elementXpath?: string;
  pageUrl?: string;
  viewportWidth?: number;
  serverId?: string;
  level?: number;
}

export interface RiskPayload {
  riskEventId?: string;
  riskType?: string;
  riskLevel?: string;
  riskScore?: number;
  ruleId?: number;
  ruleName?: string;
  ruleContext?: Record<string, unknown>;
  relatedEventIds?: string[];
  description?: string;
  evidence?: Record<string, string>;
  status?: string;
}

export interface ReportEvent {
  eventType: EventType;
  eventId: string;
  eventName: string;
  eventTime: string;
  userId?: number;
  deviceId?: string;
  sessionId?: string;
  platform?: string;
  eventCategory?: string;
  properties?: Record<string, string>;
  metrics?: Record<string, number>;
  traceId?: string;
  // 顶层字符串镜像维度（数值标量不落顶层，只进 behavior）
  eventAction?: string;
  objectType?: string;
  objectId?: string;
  objectName?: string;
  amount?: string;
  opResult?: string;
  errorCode?: string;
  behavior?: BehaviorPayload;
  risk?: RiskPayload;
}

export interface PostReportRequest {
  appId: string;
  appSecret: string;
  clientInfo?: ClientInfo;
  events: ReportEvent[];
}

export interface PostReportResponse {
  success?: boolean;
  message?: string;
  totalCount?: number;
  successCount?: number;
  failedCount?: number;
  requestId?: string;
}

export interface FlushResult {
  success: boolean;
  sent: number;
  dropped: number;
  /** 失败是否值得补发：网络错误/5xx/429 为 true；4xx 确定性失败为 false。 */
  retryable?: boolean;
  message?: string;
}

/** 单次上报维度，与其他端 SDK 的 TrackOptions 对齐。 */
export interface TrackOptions {
  eventCategory?: string;
  userId?: number;
  deviceId?: string;
  sessionId?: string;
  platform?: string;
  eventAction?: string;
  objectType?: string;
  objectId?: string;
  objectName?: string;
  durationMs?: number;
  amount?: string;
  quantity?: number;
  score?: number;
  metrics?: Record<string, number>;
  network?: string;
  opResult?: string;
  errorCode?: string;
  properties?: Record<string, string>;
  serverId?: string;
  level?: number;
  traceId?: string;
}

export interface RiskEvent extends RiskPayload {}
