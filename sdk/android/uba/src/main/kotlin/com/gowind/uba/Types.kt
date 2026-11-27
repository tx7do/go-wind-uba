package com.gowind.uba

/**
 * 上报契约类型。字段名与后端 proto（uba/service/v1/report.proto 等的 json_name）逐一对齐，
 * 序列化输出 camelCase 并跳过 null；行为事件构造时即保证数值标量只进 behavior oneof。
 */

/** 行为事件 oneof 载荷：数值标量只在这里才被服务端信任（proto3 无法区分 unset 0）。 */
data class BehaviorPayload(
    val eventAction: String? = null,
    val objectType: String? = null,
    val objectId: String? = null,
    val objectName: String? = null,
    val sessionSeq: Long? = null,
    val os: String? = null,
    val appVersion: String? = null,
    val channel: String? = null,
    val network: String? = null,
    val durationMs: Long? = null,
    val amount: String? = null,
    val quantity: Long? = null,
    val score: Int? = null,
    val metrics: Map<String, Double>? = null,
    val properties: Map<String, String>? = null,
    val opResult: String? = null,
    val errorCode: String? = null,
    val clickX: Long? = null,
    val clickY: Long? = null,
    val elementXpath: String? = null,
    val pageUrl: String? = null,
    val viewportWidth: Long? = null,
    val serverId: String? = null,
    val level: Int? = null,
)

/** 风险事件载荷（risk oneof）。riskEventId/occurTime/reportTime 服务端补全。 */
data class RiskEvent(
    val riskType: String? = null,
    val riskLevel: String? = null,
    val riskScore: Double? = null,
    val ruleId: Long? = null,
    val ruleName: String? = null,
    val ruleContext: Map<String, Any?>? = null,
    val relatedEventIds: List<String>? = null,
    val description: String? = null,
    val evidence: Map<String, String>? = null,
    val status: String? = null,
)

/** 单次上报维度，字段与其他端 SDK 的 TrackOptions 一一对应。 */
data class TrackOptions(
    val eventCategory: String? = null,
    val userId: Long? = null,
    val deviceId: String? = null,
    val sessionId: String? = null,
    val platform: String? = null,
    val eventAction: String? = null,
    val objectType: String? = null,
    val objectId: String? = null,
    val objectName: String? = null,
    val durationMs: Long? = null,
    val amount: String? = null,
    val quantity: Long? = null,
    val score: Int? = null,
    val metrics: Map<String, Double>? = null,
    val network: String? = null,
    val opResult: String? = null,
    val errorCode: String? = null,
    val properties: Map<String, String>? = null,
    val serverId: String? = null,
    val level: Int? = null,
    val traceId: String? = null,
)

/** 一次 flush 的结果。 */
data class FlushResult(
    val success: Boolean,
    val sent: Int = 0,
    val dropped: Int = 0,
    /** 失败是否值得补发：网络错误/5xx/429 为 true；4xx 确定性失败为 false。 */
    val retryable: Boolean = false,
    val message: String? = null,
)

/** 内部事件结构（内存队列与离线持久化的最小单元）。 */
internal data class ReportEvent(
    val eventType: String, // "BEHAVIOR" / "RISK"
    val eventId: String,
    val eventName: String,
    val eventTime: String,
    val userId: Long?,
    val deviceId: String?,
    val sessionId: String?,
    val platform: String?,
    val eventCategory: String?,
    val properties: Map<String, String>?,
    val metrics: Map<String, Double>?,
    val traceId: String?,
    // 顶层字符串镜像维度（数值标量不落顶层）
    val eventAction: String?,
    val objectType: String?,
    val objectId: String?,
    val objectName: String?,
    val amount: String?,
    val opResult: String?,
    val errorCode: String?,
    val behavior: BehaviorPayload?,
    val risk: RiskEvent?,
)
