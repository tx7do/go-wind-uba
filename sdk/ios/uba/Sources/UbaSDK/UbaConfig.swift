import Foundation

/// SDK 版本，用于 User-Agent；与 Package 版本保持一致。
public let sdkVersion = "1.0.0"

/// 上报平台标识（iOS / macOS 端固定值）。
public let platformIOS = "ios"
public let platformMacOS = "macos"

/// SDK 配置。默认值与其他端 SDK 对齐：批量 20 条 / 5s 定时 / 3 次重试 / 1s 退避基数 /
/// 8s 超时（必须小于服务端 10s）。
public struct UbaConfig {
    /// 应用 ID（管理后台「数据采集/应用管理」生成）。
    public var appId: String
    /// 应用密钥，鉴权在请求 body 中完成。
    public var appSecret: String
    /// collector 服务地址，如 https://collector.example.com。
    public var endpoint: String
    /// 事件上报路径。
    public var path: String
    /// AB 开关拉取路径。
    public var flagsPath: String
    /// 批量触发阈值。
    public var batchSize: Int
    /// 定时 flush 间隔（毫秒）；0 禁用。
    public var flushIntervalMs: Int
    /// 失败重试次数（总尝试 = maxRetries+1）。
    public var maxRetries: Int
    /// 单次请求超时（毫秒），必须 < 服务端 10s。
    public var timeoutMs: Int
    /// 重试退避基数（毫秒）。
    public var retryBaseDelayMs: Int
    /// 调试日志开关。
    public var debug: Bool
    /// 离线队列持久化开关（默认 UserDefaults 实现，可注入自定义 KeyValueStore）。
    public var persistQueue: Bool
    /// 持久化队列最大条数。
    public var maxPersistedEvents: Int
    /// 单批请求体字符上限；服务端对未鉴权大包直接 401，必须压在 1MiB 内。
    public var maxBatchBytes: Int
    /// 应用版本，随事件上报 behavior.appVersion。
    public var appVersion: String?
    /// 渠道标识。
    public var channel: String?
    /// 首次启动自动埋 app_launch（首装判定 = deviceId 新建）。
    public var autoTrackLaunch: Bool
    /// 页面浏览事件名。
    public var pageViewEvent: String
    /// 会话轮换超时（毫秒）：后台超过该时长回到前台开启新会话。
    public var sessionTimeoutMs: Int
    /// 采样率 0~1，按 sessionId 确定性采样。
    public var sampleRate: Double
    /// 隐私合规：授权（setConsent(true)）前所有事件静默丢弃。
    public var requireConsent: Bool
    /// 平台标识覆盖（默认自动探测 ios / macos）。
    public var platform: String?

    public init(
        appId: String,
        appSecret: String,
        endpoint: String,
        path: String = "/uba/v1/report",
        flagsPath: String = "/uba/v1/flags",
        batchSize: Int = 20,
        flushIntervalMs: Int = 5000,
        maxRetries: Int = 3,
        timeoutMs: Int = 8000,
        retryBaseDelayMs: Int = 1000,
        debug: Bool = false,
        persistQueue: Bool = true,
        maxPersistedEvents: Int = 500,
        maxBatchBytes: Int = 900 * 1024,
        appVersion: String? = nil,
        channel: String? = nil,
        autoTrackLaunch: Bool = false,
        pageViewEvent: String = "page_view",
        sessionTimeoutMs: Int = 30000,
        sampleRate: Double = 1.0,
        requireConsent: Bool = false,
        platform: String? = nil
    ) {
        precondition(!appId.isEmpty, "appId 不能为空")
        precondition(!appSecret.isEmpty, "appSecret 不能为空")
        precondition(!endpoint.isEmpty, "endpoint 不能为空")
        precondition(timeoutMs > 0 && timeoutMs < 10000, "timeoutMs 必须 > 0 且 < 10000（服务端超时 10s）")
        precondition(maxBatchBytes > 0 && maxBatchBytes < 1024 * 1024, "maxBatchBytes 必须压在 1MiB 以内")
        precondition(batchSize > 0, "batchSize 必须 > 0")
        precondition(sampleRate >= 0 && sampleRate <= 1, "sampleRate 必须在 0~1")

        self.appId = appId
        self.appSecret = appSecret
        self.endpoint = endpoint.hasSuffix("/")
            ? String(endpoint.dropLast()) : endpoint
        self.path = path
        self.flagsPath = flagsPath
        self.batchSize = batchSize
        self.flushIntervalMs = flushIntervalMs
        self.maxRetries = maxRetries
        self.timeoutMs = timeoutMs
        self.retryBaseDelayMs = retryBaseDelayMs
        self.debug = debug
        self.persistQueue = persistQueue
        self.maxPersistedEvents = maxPersistedEvents
        self.maxBatchBytes = maxBatchBytes
        self.appVersion = appVersion
        self.channel = channel
        self.autoTrackLaunch = autoTrackLaunch
        self.pageViewEvent = pageViewEvent
        self.sessionTimeoutMs = sessionTimeoutMs
        self.sampleRate = sampleRate
        self.requireConsent = requireConsent
        self.platform = platform
    }

    /// 事件上报完整地址。
    public var reportUrl: String { endpoint + path }

    /// AB 开关拉取完整地址。
    public var flagsUrl: String { endpoint + flagsPath }
}

/// 一次 flush 的结果。
public struct FlushResult {
    /// 本次 flush 是否成功（可重试失败为 false）。
    public let success: Bool
    /// 成功送达的事件数。
    public let sent: Int
    /// 未送达的事件数。
    public let dropped: Int
    /// 失败是否值得补发：网络错误/5xx/429 为 true；4xx 确定性失败为 false。
    public let retryable: Bool
    /// 失败原因。
    public let message: String?
}
