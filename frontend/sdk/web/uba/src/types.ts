/**
 * UBA SDK 类型定义
 *
 * 字段命名严格遵循后端 proto 契约（protojson 编码，全 camelCase）。
 * 对应 proto：uba/service/v1/report.proto、behavior_event.proto、risk_event.proto
 */

/** 事件类型枚举（对应 ReportEvent.EventType） */
export enum EventType {
  Behavior = 'BEHAVIOR',
  Risk = 'RISK',
}

/** 客户端信息（对应 ClientInfo，服务端用于补全 ip 城市/UA/来源） */
export interface ClientInfo {
  userAgent?: string;
  referer?: string;
  country?: string;
  city?: string;
}

/** 行为事件 payload（对应 BehaviorEvent，放在 ReportEvent.behavior oneof 内） */
export interface BehaviorEvent {
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
  opResult?: string;
  errorCode?: string;
  /** 游戏区服 ID（如 s1、cn-east-1，游戏专属维度） */
  serverId?: string;
  /** 玩家等级（游戏专属维度，事件发生时的等级快照） */
  level?: number;
  /** 点击坐标 X（相对文档，像素；autotrack 自动填充） */
  clickX?: number;
  /** 点击坐标 Y（相对文档，像素） */
  clickY?: number;
  /** 被点击元素的 XPath（如 /div[2]/section[1]/a[3]） */
  elementXpath?: string;
  /** 页面 URL（热力图按页面分组） */
  pageUrl?: string;
  /** 视口宽度（像素） */
  viewportWidth?: number;
}

/** 风险事件 payload（对应 RiskEvent，放在 ReportEvent.risk oneof 内） */
export interface RiskEvent {
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
  status?: RiskStatus;
  handlerId?: number;
  handleRemark?: string;
  occurTime?: string;
  reportTime?: string;
}

/** 风险处置状态（对应 RiskEvent.Status 枚举） */
export enum RiskStatus {
  Pending = 'PENDING',
  Investigating = 'INVESTIGATING',
  Confirmed = 'CONFIRMED',
  FalsePositive = 'FALSE_POSITIVE',
  Ignored = 'IGNORED',
  AutoBlocked = 'AUTO_BLOCKED',
}

/**
 * 统一事件结构（对应 ReportEvent）。
 *
 * 注意：
 * - eventType + behavior/risk 构成 oneof，二者必须匹配（BEHAVIOR→behavior，RISK→risk）
 * - tenantId 无需上报，服务端用应用权威值覆盖
 * - eventId/eventName/eventTime 为必填，SDK 会自动补全
 */
export interface ReportEvent {
  eventType: EventType;

  /** 事件唯一 ID（必填，SDK 自动生成 uuid） */
  eventId?: string;
  /** 登录用户 ID（未登录可不填） */
  userId?: number;
  /** 设备指纹 */
  deviceId?: string;
  /** 事件发生时间，RFC3339（必填，SDK 自动生成当前时间） */
  eventTime?: string;
  /** 事件名称（必填） */
  eventName: string;
  /** 事件大类 */
  eventCategory?: string;
  /** 会话 ID */
  sessionId?: string;
  /** 平台：web/ios/android 等 */
  platform?: string;
  /** 客户端 IP（通常由服务端解析，可不填） */
  ip?: string;
  /** 自定义属性（落入服务端 context 列） */
  properties?: Record<string, string>;
  /** 链路追踪 ID */
  traceId?: string;

  /** 事件动作 */
  eventAction?: string;
  /** 对象类型 */
  objectType?: string;
  /** 对象 ID */
  objectId?: string;
  /** 对象名称 */
  objectName?: string;
  /** 会话内序号 */
  sessionSeq?: number;
  /** 操作系统 */
  os?: string;
  /** 应用版本 */
  appVersion?: string;
  /** 渠道 */
  channel?: string;
  /** 网络类型 */
  network?: string;
  /** 持续时间（毫秒） */
  durationMs?: number;
  /** 金额（字符串，兼容货币格式） */
  amount?: string;
  /** 数量 */
  quantity?: number;
  /** 分数 */
  score?: number;
  /** 数值指标 */
  metrics?: Record<string, number>;
  /** 操作结果 */
  opResult?: string;
  /** 错误码 */
  errorCode?: string;
  /** 游戏区服 ID */
  serverId?: string;
  /** 玩家等级 */
  level?: number;
  /** 点击坐标 X（相对文档，像素；autotrack 自动填充） */
  clickX?: number;
  /** 点击坐标 Y（相对文档，像素） */
  clickY?: number;
  /** 被点击元素的 XPath */
  elementXpath?: string;
  /** 页面 URL */
  pageUrl?: string;
  /** 视口宽度（像素） */
  viewportWidth?: number;

  /** oneof payload：BEHAVIOR 事件填此项 */
  behavior?: BehaviorEvent;
  /** oneof payload：RISK 事件填此项 */
  risk?: RiskEvent;
}

/** 上报请求体（对应 PostReportRequest） */
export interface PostReportRequest {
  appId: string;
  appSecret: string;
  events: ReportEvent[];
  clientInfo?: ClientInfo;
}

/** 单条错误详情 */
export interface ErrorDetail {
  code: string;
  message: string;
  eventId?: string;
}

/** 按事件类型分组的错误 */
export interface TypeErrorDetail {
  type: string;
  errors: ErrorDetail[];
}

/** 上报响应体（对应 PostReportResponse） */
export interface PostReportResponse {
  success: boolean;
  message: string;
  errorsByType?: TypeErrorDetail[];
  requestId?: string;
  serverTime?: number;
  totalCount?: number;
  successCount?: number;
  failedCount?: number;
}

/** Kratos 标准错误信封（非 200 响应体） */
export interface KratosError {
  code: number;
  reason: string;
  message: string;
  metadata?: Record<string, unknown>;
}

/** SDK 配置项 */
export interface UbaConfig {
  /** 应用 ID（必填，鉴权用） */
  appId: string;
  /** 应用密钥（必填，鉴权用） */
  appSecret: string;
  /** collector 服务地址，如 http://localhost:5700 */
  endpoint: string;
  /** 上报路径，默认 /uba/v1/report */
  path?: string;
  /** 缓冲达到该数量时触发批量上报，默认 20 */
  batchSize?: number;
  /** 定时上报间隔（毫秒），默认 5000 */
  flushInterval?: number;
  /** 失败最大重试次数，默认 3 */
  maxRetries?: number;
  /** 单次请求超时（毫秒），默认 8000（须小于服务端 10s） */
  timeout?: number;
  /** 重试基础退避（毫秒），默认 1000，按指数递增 */
  retryBaseDelay?: number;
  /** 是否开启调试日志，默认 false */
  debug?: boolean;
  /** 是否在页面卸载时用 sendBeacon 兜底，默认 true */
  enableBeacon?: boolean;
  /** 是否开启自动埋点（监听 click 自动上报点击事件），默认 true */
  autoTrack?: boolean;
}

/** track() 的可选参数 */
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
  /** 自定义属性，并入 properties */
  properties?: Record<string, string>;
  /** 游戏区服 ID */
  serverId?: string;
  /** 玩家等级 */
  level?: number;
  /** 点击坐标 X（相对文档，像素） */
  clickX?: number;
  /** 点击坐标 Y（相对文档，像素） */
  clickY?: number;
  /** 被点击元素的 XPath */
  elementXpath?: string;
  /** 页面 URL */
  pageUrl?: string;
  /** 视口宽度（像素） */
  viewportWidth?: number;
}
