package com.gowind.uba

/** 上报平台标识（服务端按 UA/平台字段区分来源），Android 端固定值。 */
const val PLATFORM_ANDROID = "android"

/** SDK 版本，用于 User-Agent；与 Gradle 版本保持一致。 */
const val SDK_VERSION = "1.0.0"

/**
 * SDK 配置。默认值与其他端 SDK 对齐：批量 20 条 / 5s 定时 / 3 次重试 / 1s 退避基数 /
 * 8s 超时（必须小于服务端 10s）。
 */
data class UbaConfig(
    /** 应用 ID（管理后台「数据采集/应用管理」生成）。 */
    val appId: String,
    /** 应用密钥，鉴权在请求 body 中完成。 */
    val appSecret: String,
    /** collector 服务地址，如 https://collector.example.com。 */
    val endpoint: String,
    /** 事件上报路径。 */
    val path: String = "/uba/v1/report",
    /** AB 开关拉取路径。 */
    val flagsPath: String = "/uba/v1/flags",
    /** 批量触发阈值。 */
    val batchSize: Int = 20,
    /** 定时 flush 间隔（毫秒）；0 禁用。 */
    val flushIntervalMs: Long = 5000,
    /** 失败重试次数（总尝试 = maxRetries+1）。 */
    val maxRetries: Int = 3,
    /** 单次请求超时（毫秒），必须 < 服务端 10s。 */
    val timeoutMs: Int = 8000,
    /** 重试退避基数（毫秒），第 n 次重试延迟 = retryBaseDelayMs * 2^n。 */
    val retryBaseDelayMs: Long = 1000,
    /** 调试日志开关（logcat 以 "uba-android" 为 tag）。 */
    val debug: Boolean = false,
    /** 离线队列持久化开关（通过 [com.gowind.uba.KeyValueStore]）。 */
    val persistQueue: Boolean = true,
    /** 持久化队列最大条数。 */
    val maxPersistedEvents: Int = 500,
    /** 单批请求体字符上限；服务端对未鉴权大包直接 401，必须压在 1MiB 内。 */
    val maxBatchBytes: Int = 900 * 1024,
    /** 应用版本，随事件上报 behavior.appVersion。 */
    val appVersion: String? = null,
    /** 渠道标识。 */
    val channel: String? = null,
    /** 首次启动自动埋 app_launch（首装判定 = deviceId 新建）。 */
    val autoTrackLaunch: Boolean = false,
    /** 页面浏览事件名。 */
    val pageViewEvent: String = "page_view",
    /** 会话轮换超时（毫秒）：后台超过该时长回到前台开启新会话。 */
    val sessionTimeoutMs: Long = 30000,
    /** 采样率 0~1，按 sessionId 确定性采样。 */
    val sampleRate: Double = 1.0,
    /** 隐私合规：授权（setConsent(true)）前所有事件静默丢弃。 */
    val requireConsent: Boolean = false,
) {
    init {
        require(appId.isNotEmpty()) { "appId 不能为空" }
        require(appSecret.isNotEmpty()) { "appSecret 不能为空" }
        require(endpoint.isNotEmpty()) { "endpoint 不能为空" }
        require(timeoutMs in 1..9999) { "timeoutMs 必须 > 0 且 < 10000（服务端超时 10s）" }
        require(maxBatchBytes in 1 until 1024 * 1024) { "maxBatchBytes 必须压在 1MiB 以内" }
        require(batchSize > 0) { "batchSize 必须 > 0" }
        require(sampleRate in 0.0..1.0) { "sampleRate 必须在 0~1" }
    }

    /** 规整尾斜杠后的 endpoint。 */
    val baseEndpoint: String get() = endpoint.removeSuffix("/")

    /** 事件上报完整地址。 */
    val reportUrl: String get() = baseEndpoint + path

    /** AB 开关拉取完整地址。 */
    val flagsUrl: String get() = baseEndpoint + flagsPath
}

/** 重试退避延迟：第 attempt 次重试（从 0 计）的毫秒数。 */
internal fun backoffDelayMs(baseMs: Long, attempt: Int): Long = baseMs shl attempt.coerceAtMost(30)
