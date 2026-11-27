import Foundation

// MARK: - 上报契约模型
// 铁律：tenantId/serverTime/ip 服务端补全不发送；数值标量只进 behavior oneof。

/// 风险事件载荷（risk oneof）。riskEventId / occurTime / reportTime 服务端补全。
public struct RiskEvent {
    public var riskType: String?
    public var riskLevel: String?
    public var riskScore: Double?
    public var ruleId: Int64?
    public var ruleName: String?
    /// 规则上下文（任意 JSON 对象，对应 google.protobuf.Struct）。
    public var ruleContext: [String: Any]? = nil
    public var relatedEventIds: [String]?
    public var description: String?
    public var evidence: [String: String]?
    /// 处置状态：PENDING / INVESTIGATING / CONFIRMED / FALSE_POSITIVE / IGNORED / AUTO_BLOCKED。
    public var status: String?

    public init(
        riskType: String? = nil,
        riskLevel: String? = nil,
        riskScore: Double? = nil,
        ruleId: Int64? = nil,
        ruleName: String? = nil,
        ruleContext: [String: Any]? = nil,
        relatedEventIds: [String]? = nil,
        description: String? = nil,
        evidence: [String: String]? = nil,
        status: String? = nil
    ) {
        self.riskType = riskType
        self.riskLevel = riskLevel
        self.riskScore = riskScore
        self.ruleId = ruleId
        self.ruleName = ruleName
        self.ruleContext = ruleContext
        self.relatedEventIds = relatedEventIds
        self.description = description
        self.evidence = evidence
        self.status = status
    }
}

/// 单次上报维度，字段与其他端 SDK 的 TrackOptions 一一对应。
public struct TrackOptions {
    public var eventCategory: String?
    public var userId: Int64?
    public var deviceId: String?
    public var sessionId: String?
    public var platform: String?
    public var eventAction: String?
    public var objectType: String?
    public var objectId: String?
    public var objectName: String?
    public var durationMs: Int64?
    public var amount: String?
    public var quantity: Int64?
    public var score: Int?
    public var metrics: [String: Double]?
    public var network: String?
    public var opResult: String?
    public var errorCode: String?
    public var properties: [String: String]?
    public var serverId: String?
    public var level: Int?
    public var traceId: String?

    public init(
        eventCategory: String? = nil,
        userId: Int64? = nil,
        deviceId: String? = nil,
        sessionId: String? = nil,
        platform: String? = nil,
        eventAction: String? = nil,
        objectType: String? = nil,
        objectId: String? = nil,
        objectName: String? = nil,
        durationMs: Int64? = nil,
        amount: String? = nil,
        quantity: Int64? = nil,
        score: Int? = nil,
        metrics: [String: Double]? = nil,
        network: String? = nil,
        opResult: String? = nil,
        errorCode: String? = nil,
        properties: [String: String]? = nil,
        serverId: String? = nil,
        level: Int? = nil,
        traceId: String? = nil
    ) {
        self.eventCategory = eventCategory
        self.userId = userId
        self.deviceId = deviceId
        self.sessionId = sessionId
        self.platform = platform
        self.eventAction = eventAction
        self.objectType = objectType
        self.objectId = objectId
        self.objectName = objectName
        self.durationMs = durationMs
        self.amount = amount
        self.quantity = quantity
        self.score = score
        self.metrics = metrics
        self.network = network
        self.opResult = opResult
        self.errorCode = errorCode
        self.properties = properties
        self.serverId = serverId
        self.level = level
        self.traceId = traceId
    }
}

/// 内部事件结构（队列与离线持久化按 wire JSON 原样保存，恢复后直接重发，无损）。
struct ReportEvent {
    var eventType: String // "BEHAVIOR" / "RISK"
    var eventId: String
    var eventName: String
    var eventTime: String
    var userId: Int64?
    var deviceId: String?
    var sessionId: String?
    var platform: String?
    var eventCategory: String?
    var properties: [String: String]?
    var metrics: [String: Double]?
    var traceId: String?
    // 顶层字符串镜像维度（数值标量不落顶层）
    var eventAction: String?
    var objectType: String?
    var objectId: String?
    var objectName: String?
    var amount: String?
    var opResult: String?
    var errorCode: String?
    var behavior: BehaviorPayload?
    var risk: RiskEvent?
}

/// 行为事件 oneof 载荷：数值标量只在这里才被服务端信任（proto3 无法区分 unset 0）。
struct BehaviorPayload {
    var eventAction: String?
    var objectType: String?
    var objectId: String?
    var objectName: String?
    var sessionSeq: Int64?
    var os: String?
    var appVersion: String?
    var channel: String?
    var network: String?
    var durationMs: Int64?
    var amount: String?
    var quantity: Int64?
    var score: Int?
    var metrics: [String: Double]?
    var opResult: String?
    var errorCode: String?
    var serverId: String?
    var level: Int?
}

// MARK: - 工具

/// UTC 时间 → RFC3339 毫秒精度字符串（与其他端 SDK 精度一致）。
func toRFC3339Ms(_ date: Date) -> String {
    let fmt = DateFormatter()
    fmt.dateFormat = "yyyy-MM-dd'T'HH:mm:ss.SSS'Z'"
    fmt.timeZone = TimeZone(identifier: "UTC")
    fmt.locale = Locale(identifier: "en_US_POSIX")
    return fmt.string(from: date)
}

/// FNV-1a 32 位哈希：按 sessionId 确定性采样（与 Web/Flutter/Go SDK 算法一致）。
func fnv1a32(_ s: String) -> UInt32 {
    var hash: UInt32 = 0x811c9dc5
    for byte in s.utf8 {
        hash ^= UInt32(byte)
        hash = hash &* 16777619
    }
    return hash
}

/// 会话是否落在采样区间：hash(sessionId) % 1000 / 1000 < sampleRate。
func sessionIdInSample(_ sessionId: String, _ sampleRate: Double) -> Bool {
    if sampleRate >= 1 { return true }
    if sampleRate <= 0 { return false }
    return Double(fnv1a32(sessionId) % 1000) / 1000.0 < sampleRate
}

/// 重试退避延迟：第 attempt 次重试（从 0 计）的毫秒数。
func backoffDelayMs(_ baseMs: Int, _ attempt: Int) -> Int {
    return baseMs * (1 << min(attempt, 30))
}
