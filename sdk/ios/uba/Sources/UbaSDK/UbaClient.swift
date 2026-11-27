import Foundation

// MARK: - 存储与传输抽象

/// 键值存储抽象：隔离 UserDefaults，测试可注入内存实现。键名沿用 `__uba_*__` 约定。
public protocol KeyValueStore: AnyObject {
    func getString(_ key: String) -> String?
    func setString(_ key: String, _ value: String)
    func remove(_ key: String)
}

/// 设备 ID 持久化键（对齐 Web localStorage / Unity PlayerPrefs / Flutter 约定）。
public let deviceIdKey = "__uba_device_id__"
/// 离线队列持久化键（对齐 Web localStorage）。
public let offlineQueueKey = "__uba_offline_queue__"

/// UserDefaults 实现（iOS/macOS 通用）。
public final class UserDefaultsStore: KeyValueStore {
    private let defaults: UserDefaults
    public init(_ defaults: UserDefaults = .standard) {
        self.defaults = defaults
    }
    public func getString(_ key: String) -> String? {
        defaults.string(forKey: key)
    }
    public func setString(_ key: String, _ value: String) {
        defaults.set(value, forKey: key)
    }
    public func remove(_ key: String) {
        defaults.removeObject(forKey: key)
    }
}

/// 内存实现：测试与临时客户端用（进程退出即丢）。
public final class InMemoryStore: KeyValueStore {
    private var data: [String: String] = [:]
    private let lock = NSLock()
    public init() {}
    public func getString(_ key: String) -> String? {
        lock.lock(); defer { lock.unlock() }
        return data[key]
    }
    public func setString(_ key: String, _ value: String) {
        lock.lock(); defer { lock.unlock() }
        data[key] = value
    }
    public func remove(_ key: String) {
        lock.lock(); defer { lock.unlock() }
        data.removeValue(forKey: key)
    }
}

/// HTTP 传输结果：statusCode 0 表示网络层失败（超时/连接拒绝），可重试。
public struct HttpResult {
    public let statusCode: Int
    public let body: String
    public var isOk: Bool { statusCode >= 200 && statusCode < 300 }
}

/// HTTP 传输抽象：默认 URLSession；换网关/加签时实现本接口注入。
public protocol HttpTransport: AnyObject {
    func send(_ url: String, _ body: String, _ timeoutMs: Int, completion: @escaping (HttpResult) -> Void)
    func close()
}

/// URLSession 默认实现。
public final class URLSessionTransport: HttpTransport {
    private let session: URLSession
    public init() {
        let config = URLSessionConfiguration.ephemeral
        config.requestCachePolicy = .reloadIgnoringLocalCacheData
        session = URLSession(configuration: config)
    }
    public func send(_ url: String, _ body: String, _ timeoutMs: Int,
                     completion: @escaping (HttpResult) -> Void) {
        guard let urlObj = URL(string: url) else {
            completion(HttpResult(statusCode: 0, body: "invalid url"))
            return
        }
        var request = URLRequest(url: urlObj, timeoutInterval: TimeInterval(timeoutMs) / 1000.0)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = body.data(using: .utf8)
        session.dataTask(with: request) { data, response, error in
            if let error = error {
                completion(HttpResult(statusCode: 0, body: error.localizedDescription))
                return
            }
            let status = (response as? HTTPURLResponse)?.statusCode ?? 0
            let text = String(data: data ?? Data(), encoding: .utf8) ?? ""
            completion(HttpResult(statusCode: status, body: text))
        }.resume()
    }
    public func close() {
        session.finishTasksAndInvalidate()
    }
}

// MARK: - 会话管理

/// 会话管理：进程内 uuid，后台超过 timeoutMs 后回到前台时轮换新会话。
public final class SessionManager {
    private let timeoutMs: Int
    private let now: () -> Int64
    private var id: String = UbaClient.newUuid()
    private var pausedAt: Int64?

    public init(timeoutMs: Int = 30000, now: @escaping () -> Int64 = { Int64(Date().timeIntervalSince1970 * 1000) }) {
        self.timeoutMs = timeoutMs
        self.now = now
    }

    public var current: String { id }

    public func inSample(_ sampleRate: Double) -> Bool {
        sessionIdInSample(id, sampleRate)
    }

    /// 进入后台（applicationDidEnterBackground）。
    public func onBackground() {
        if pausedAt == nil { pausedAt = now() }
    }

    /// 回到前台（applicationWillEnterForeground）：超时则轮换新会话。
    public func onForeground() {
        let paused = pausedAt
        pausedAt = nil
        if let paused = pausedAt, timeoutMs > 0, now() - paused > Int64(timeoutMs) {
            rotate()
        }
    }

    /// 手动开启新会话。
    public func rotate() {
        id = UbaClient.newUuid()
        pausedAt = nil
    }
}

// MARK: - 客户端

/// GoWind UBA iOS/macOS 埋点客户端。线程安全（内部串行队列 + 锁）。
///
/// iOS 宿主在 AppDelegate 接入前后台：
/// ```
/// applicationDidEnterBackground -> client.handleBackground()
/// applicationWillEnterForeground -> client.handleForeground()
/// ```
public final class UbaClient {

    public let config: UbaConfig
    private let store: KeyValueStore
    private let transport: HttpTransport
    private let queue = DispatchQueue(label: "com.gowind.uba.client")
    private var wireQueue: [String] = [] // wire JSON 事件（内存队列 + 离线恢复共用保真格式）
    private var restoredWire: [String] = [] // 离线盘恢复的事件（最旧优先）
    private var flushing = false
    private var closed = false

    private let session: SessionManager
    private var consentGranted = false
    private var superProps: [String: String] = [:]
    private var flags: [String: String] = [:]
    private var exposedFlags: Set<String> = []
    private var exposedSessionId: String?
    private var userId: Int64?
    private var lastScreenAt: Int64?
    private var timer: DispatchSourceTimer?

    private let deviceId: String
    private let deviceIdIsNew: Bool
    private let userAgent: String

    /// 初始化客户端（恢复离线队列并启动定时 flush）。
    public init(config: UbaConfig, store: KeyValueStore = UserDefaultsStore(),
                transport: HttpTransport = URLSessionTransport()) {
        self.config = config
        self.store = store
        self.transport = transport
        self.session = SessionManager(timeoutMs: config.sessionTimeoutMs)

        let persisted = store.getString(deviceIdKey)
        deviceIdIsNew = (persisted ?? "").isEmpty
        deviceId = deviceIdIsNew ? UbaClient.newUuid() : persisted!
        if deviceIdIsNew {
            store.setString(deviceIdKey, deviceId)
        }
        let osName: String
        #if targetEnvironment(macCatalyst) || os(macOS)
        osName = "macOS"
        #else
        osName = "iOS"
        #endif
        userAgent = "uba-swift/\(sdkVersion) (\(osName))"

        restorePersisted()
        startTimer()

        if config.autoTrackLaunch && deviceIdIsNew {
            track("app_launch", properties: ["first_launch": "true"],
                  options: TrackOptions(eventCategory: "app"))
        }
    }

    // MARK: 上报 API

    /// 上报自定义行为事件。可重试失败留在队列等待补发。
    public func track(_ eventName: String,
                      properties: [String: String]? = nil,
                      options: TrackOptions? = nil) {
        trackBehavior(eventName, properties: properties, options: options)
    }

    /// 上报自定义行为事件。
    public func trackBehavior(_ eventName: String,
                              properties: [String: String]? = nil,
                              options: TrackOptions? = nil) {
        let event = buildBehaviorEvent(eventName, properties, options)
        enqueue(event)
    }

    /// 上报风险事件（进入 uba_risk_events 通道）。
    public func trackRisk(_ eventName: String,
                          _ risk: RiskEvent,
                          properties: [String: String]? = nil,
                          options: TrackOptions? = nil) {
        let event = ReportEvent(
            eventType: "RISK",
            eventId: UbaClient.newUuid(),
            eventName: eventName,
            eventTime: toRFC3339Ms(Date()),
            userId: options?.userId ?? userId,
            deviceId: options?.deviceId ?? deviceId,
            sessionId: options?.sessionId ?? session.current,
            platform: options?.platform ?? (config.platform ?? platformIOS),
            eventCategory: options?.eventCategory,
            properties: mergedProperties(properties, options),
            metrics: options?.metrics,
            traceId: options?.traceId,
            eventAction: nil, objectType: nil, objectId: nil, objectName: nil,
            amount: nil, opResult: nil, errorCode: nil,
            behavior: nil, risk: risk
        )
        enqueue(event)
    }

    /// 页面浏览事件：自动携带上一页面停留时长 durationMs。
    public func trackPageView(_ screenName: String, title: String? = nil, referer: String? = nil) {
        let now = Int64(Date().timeIntervalSince1970 * 1000)
        let dwell = lastScreenAt.map { now - $0 }
        lastScreenAt = now
        var props = ["pageUrl": screenName]
        if let title = title, !title.isEmpty { props["title"] = title }
        if let referer = referer, !referer.isEmpty { props["referer"] = referer }
        trackBehavior(
            config.pageViewEvent,
            properties: props,
            options: TrackOptions(
                eventCategory: "navigation",
                objectType: "page",
                objectId: screenName,
                durationMs: dwell.flatMap { $0 > 0 ? $0 : nil }
            )
        )
    }

    /// 登录绑定用户 ID。
    public func identify(_ id: Int64) {
        queue.sync { userId = id }
    }

    /// 登出清空用户绑定。
    public func resetUser() {
        queue.sync { userId = nil }
    }

    /// 设置全局公共属性：合并进之后每个事件（同名覆盖）。
    public func setSuperProperties(_ props: [String: String]) {
        queue.sync { superProps.merge(props) { _, new in new } }
    }

    /// 清空全局公共属性。
    public func clearSuperProperties() {
        queue.sync { superProps.removeAll() }
    }

    /// 隐私合规授权开关；requireConsent = true 时未授权前所有上报静默丢弃。
    public func setConsent(_ granted: Bool) {
        queue.sync { consentGranted = granted }
    }

    /// 当前是否处于采集状态（未关闭、consent 放行、会话命中采样）。
    public func isCollecting() -> Bool {
        queue.sync {
            !closed && consentOk() && session.inSample(config.sampleRate)
        }
    }

    // MARK: 生命周期

    /// 退到后台：立即 flush + 记录时刻。
    public func handleBackground() {
        session.onBackground()
        _ = flush()
    }

    /// 回到前台：后台超过 sessionTimeoutMs 则轮换新会话。
    public func handleForeground() {
        session.onForeground()
    }

    // MARK: AB 实验开关

    /// 拉取最新实验开关并缓存。失败保留旧缓存（回调带当前缓存）。
    public func loadFlags(completion: @escaping ([String: String]) -> Void) {
        var bodyParts: [String] = [
            "\"appId\":\(UbaJson.str(config.appId))",
            "\"appSecret\":\(UbaJson.str(config.appSecret))",
            "\"deviceId\":\(UbaJson.str(deviceId))",
        ]
        if let uid = userId {
            bodyParts.append("\"userId\":\(uid)")
        }
        let body = "{" + bodyParts.joined(separator: ",") + "}"
        transport.send(config.flagsUrl, body, config.timeoutMs) { [weak self] result in
            guard let self = self else { return }
            if result.isOk {
                let parsed = Self.parseFlags(result.body)
                self.queue.sync { self.flags = parsed }
            } else {
                self.debugLog("拉取 flags 失败：HTTP \(result.statusCode)")
            }
            completion(self.queue.sync { self.flags })
        }
    }

    /// 读取开关变体；命中时自动上报一次 $feature_flag 曝光（每会话每开关一次）。
    public func getFlag(_ key: String) -> String? {
        let variant = queue.sync { flags[key] }
        if let variant = variant {
            trackFlagExposure(key, variant)
        }
        return variant
    }

    /// 变体非空且非 control 即视为命中（不触发曝光事件）。
    public func isFeatureEnabled(_ key: String) -> Bool {
        guard let v = queue.sync({ flags[key] }) else { return false }
        return !v.isEmpty && v != "control"
    }

    // MARK: 维护

    /// 队列中待上报条数（含离线恢复的）。
    public func pendingCount() -> Int {
        queue.sync { wireQueue.count + restoredWire.count }
    }

    /// 手动冲刷队列：分批发送直到清空或遇到失败。并发调用安全（重入直接返回）。
    /// 同步实现：内部用信号量等待网络回调（对齐 Java/Kotlin 端语义）。
    @discardableResult
    public func flush() -> FlushResult {
        if closed {
            return FlushResult(success: true, sent: 0, dropped: 0, retryable: false, message: nil)
        }
        let shouldRun: Bool = queue.sync {
            if flushing { return false }
            flushing = true
            return true
        }
        guard shouldRun else {
            return FlushResult(success: true, sent: 0, dropped: 0, retryable: false, message: nil)
        }

        var sent = 0
        var dropped = 0
        var ok = true
        var retryable = false
        var message: String?
        var drainingRestored = true

        while true {
            var fromRestored = false
            let batch: [String]? = queue.sync {
                func take(_ source: inout [String]) -> [String] {
                    var taken: [String] = []
                    var bytes = 0
                    while !source.isEmpty && taken.count < config.batchSize {
                        let next = source[0]
                        if !taken.isEmpty && bytes + next.count > config.maxBatchBytes { break }
                        bytes += next.count
                        taken.append(next)
                        source.removeFirst()
                    }
                    return taken
                }
                if drainingRestored && !restoredWire.isEmpty {
                    fromRestored = true
                    return take(&restoredWire)
                }
                drainingRestored = false
                if !wireQueue.isEmpty {
                    fromRestored = false
                    return take(&wireQueue)
                }
                return nil
            }
            guard let b = batch else { break }

            let semaphore = DispatchSemaphore(value: 0)
            var result: FlushResult?
            sendWithRetry(b) { r in
                result = r
                semaphore.signal()
            }
            semaphore.wait()
            guard let r = result else { break }
            sent += r.sent
            dropped += r.dropped
            if !r.success {
                ok = false
                retryable = r.retryable
                message = r.message
                // 可重试失败回到对应队列队首等下次补发；确定性 4xx 直接丢弃
                if r.retryable {
                    queue.sync {
                        if fromRestored {
                            restoredWire = b + restoredWire
                        } else {
                            wireQueue = b + wireQueue
                        }
                    }
                }
                break
            }
        }

        persist()
        queue.sync { flushing = false }
        return FlushResult(success: ok, sent: sent, dropped: dropped, retryable: retryable, message: message)
    }

    /// 关闭：停止定时器并冲刷剩余队列。之后 track 不再入队。
    @discardableResult
    public func close() -> FlushResult {
        queue.sync {
            timer?.cancel()
            timer = nil
        }
        let result = flush()
        queue.sync { closed = true }
        transport.close()
        return result
    }

    // MARK: 内部实现

    private func startTimer() {
        guard config.flushIntervalMs > 0 else { return }
        let t = DispatchSource.makeTimerSource(queue: queue)
        t.schedule(deadline: .now() + Double(config.flushIntervalMs) / 1000.0,
                   repeating: Double(config.flushIntervalMs) / 1000.0)
        t.setEventHandler { [weak self] in
            guard let self = self else { return }
            let hasPending = self.queue.sync { !self.wireQueue.isEmpty || !self.restoredWire.isEmpty }
            if hasPending && !self.closed {
                _ = self.flush()
            }
        }
        t.resume()
        timer = t
    }

    private func consentOk() -> Bool {
        !config.requireConsent || consentGranted
    }

    private func buildBehaviorEvent(_ eventName: String,
                                    _ properties: [String: String]?,
                                    _ options: TrackOptions?) -> ReportEvent? {
        let snapshot: (consentOk: Bool, sampled: Bool, userId: Int64?,
                       superProps: [String: String]) = queue.sync {
            (consentOk(), session.inSample(config.sampleRate), userId, superProps)
        }
        if closed {
            debugLog("客户端已关闭，丢弃事件 \(eventName)")
            return nil
        }
        if !snapshot.consentOk {
            debugLog("consent 未授权，丢弃事件 \(eventName)")
            return nil
        }
        if !snapshot.sampled { return nil } // 会话级确定性采样

        let o = options
        // 数值标量只进 behavior oneof；顶层只镜像字符串维度
        let behavior = BehaviorPayload(
            eventAction: o?.eventAction,
            objectType: o?.objectType,
            objectId: o?.objectId,
            objectName: o?.objectName,
            os: currentOS(),
            appVersion: config.appVersion,
            channel: config.channel,
            network: o?.network,
            durationMs: o?.durationMs,
            amount: o?.amount,
            quantity: o?.quantity,
            score: o?.score,
            metrics: o?.metrics,
            opResult: o?.opResult,
            errorCode: o?.errorCode,
            serverId: o?.serverId,
            level: o?.level
        )
        var merged = snapshot.superProps
        properties?.forEach { merged[$0.key] = $0.value }
        o?.properties?.forEach { merged[$0.key] = $0.value }

        return ReportEvent(
            eventType: "BEHAVIOR",
            eventId: UbaClient.newUuid(),
            eventName: eventName,
            eventTime: toRFC3339Ms(Date()),
            userId: o?.userId ?? snapshot.userId,
            deviceId: o?.deviceId ?? deviceId,
            sessionId: o?.sessionId ?? session.current,
            platform: o?.platform ?? (config.platform ?? platformIOS),
            eventCategory: o?.eventCategory,
            properties: merged.isEmpty ? nil : merged,
            metrics: o?.metrics,
            traceId: o?.traceId,
            eventAction: o?.eventAction,
            objectType: o?.objectType,
            objectId: o?.objectId,
            objectName: o?.objectName,
            amount: o?.amount,
            opResult: o?.opResult,
            errorCode: o?.errorCode,
            behavior: behavior,
            risk: nil
        )
    }

    private func mergedProperties(_ properties: [String: String]?, _ options: TrackOptions?) -> [String: String]? {
        var merged = queue.sync { superProps }
        properties?.forEach { merged[$0.key] = $0.value }
        options?.properties?.forEach { merged[$0.key] = $0.value }
        return merged.isEmpty ? nil : merged
    }

    private func enqueue(_ event: ReportEvent) {
        let wire = UbaJson.eventToWire(event)
        let shouldFlush: Bool = queue.sync {
            wireQueue.append(wire)
            return wireQueue.count >= config.batchSize
        }
        if shouldFlush {
            DispatchQueue.global(qos: .utility).async { [weak self] in
                _ = self?.flush()
            }
        }
    }

    private func sendWithRetry(_ batch: [String], completion: @escaping (FlushResult) -> Void) {
        let body = UbaJson.envelope(config.appId, config.appSecret, userAgent, batch)
        func attempt(_ n: Int) {
            transport.send(config.reportUrl, body, config.timeoutMs) { [weak self] res in
                guard let self = self else { return }
                if res.isOk {
                    let failed = Self.parseFailedCount(res.body)
                    if failed > 0 { self.debugLog("部分事件被服务端拒绝：HTTP 200 failedCount=\(failed)") }
                    completion(FlushResult(success: true, sent: batch.count - failed, dropped: failed,
                                           retryable: false, message: nil))
                    return
                }
                // 4xx（除 429）是确定性失败：整批丢弃，不重试
                if res.statusCode >= 400 && res.statusCode < 500 && res.statusCode != 429 {
                    self.debugLog("HTTP \(res.statusCode)（不可重试），丢弃 \(batch.count) 条")
                    completion(FlushResult(success: false, sent: 0, dropped: batch.count,
                                           retryable: false, message: "HTTP \(res.statusCode)"))
                    return
                }
                if n >= self.config.maxRetries {
                    self.debugLog("重试耗尽（HTTP \(res.statusCode)），\(batch.count) 条待落盘补发")
                    completion(FlushResult(success: false, sent: 0, dropped: batch.count,
                                           retryable: true,
                                           message: "HTTP \(res.statusCode) after \(n + 1) attempts"))
                    return
                }
                let delay = backoffDelayMs(self.config.retryBaseDelayMs, n)
                self.debugLog("HTTP \(res.statusCode)，\(delay)ms 后第 \(n + 1) 次重试")
                // 不能在 client 串行队列上调度重试：flush 可能正被该队列阻塞（信号量等待）
                DispatchQueue.global(qos: .utility).asyncAfter(
                    deadline: .now() + Double(delay) / 1000.0
                ) {
                    attempt(n + 1)
                }
            }
        }
        attempt(0)
    }

    private func restorePersisted() {
        guard let raw = store.getString(offlineQueueKey), !raw.isEmpty else { return }
        guard let events = Self.parseWireList(raw) else {
            debugLog("离线队列解析失败，已忽略")
            store.remove(offlineQueueKey)
            return
        }
        queue.sync {
            restoredWire = events + restoredWire
        }
        store.remove(offlineQueueKey)
        debugLog("离线队列恢复 \(events.count) 条")
    }

    private func persist() {
        let snapshot: [String] = queue.sync { restoredWire + wireQueue }
        if snapshot.isEmpty {
            store.remove(offlineQueueKey)
            return
        }
        let capped = snapshot.suffix(config.maxPersistedEvents)
        store.setString(offlineQueueKey, "[" + capped.joined(separator: ",") + "]")
    }

    private func trackFlagExposure(_ key: String, _ variant: String) {
        let sessionId = session.current
        let first: Bool = queue.sync {
            if exposedSessionId != sessionId {
                exposedFlags.removeAll()
                exposedSessionId = sessionId
            }
            return exposedFlags.insert(key).inserted
        }
        if first {
            track("$feature_flag",
                  properties: ["flag_key": key, "variant": variant],
                  options: TrackOptions(eventCategory: "experiment"))
        }
    }

    private func currentOS() -> String {
        #if targetEnvironment(macCatalyst) || os(macOS)
        return "macOS"
        #else
        return "iOS"
        #endif
    }

    private func debugLog(_ message: String) {
        if config.debug {
            print("[uba-swift] \(message)")
        }
    }

    // MARK: 静态工具

    static func newUuid() -> String {
        UUID().uuidString.lowercased()
    }

    /// 从 {"success":true,"failedCount":2} 轻量提取 failedCount（避免引 JSON 库）。
    static func parseFailedCount(_ body: String) -> Int {
        guard let range = body.range(of: "\"failedCount\"") else { return 0 }
        guard let colon = body.range(of: ":", range: range.upperBound..<body.endIndex) else { return 0 }
        var digits = ""
        for ch in body[colon.upperBound...] {
            if ch.isNumber {
                digits.append(ch)
            } else if !digits.isEmpty || !ch.isWhitespace {
                break
            }
        }
        return Int(digits) ?? 0
    }

    /// 从 {"flags":{"k":"v"}} 轻量提取键值对。
    static func parseFlags(_ body: String) -> [String: String] {
        guard let flagsRange = body.range(of: "\"flags\"") else { return [:] }
        guard let objStart = body.range(of: "{", range: flagsRange.upperBound..<body.endIndex) else { return [:] }
        var depth = 0
        var end = body.endIndex
        for i in objStart.lowerBound..<body.endIndex {
            let ch = body[i]
            if ch == "{" { depth += 1 }
            if ch == "}" {
                depth -= 1
                if depth == 0 {
                    end = body.index(after: i)
                    break
                }
            }
        }
        if end == body.endIndex { return [:] }
        let inner = String(body[body.index(after: objStart.lowerBound)..<body.index(before: end)])
        var out: [String: String] = [:]
        var i = inner.startIndex
        while i < inner.endIndex {
            guard let keyStart = inner.range(of: "\"", range: i..<inner.endIndex),
                  let keyEnd = inner.range(of: "\"", range: keyStart.upperBound..<inner.endIndex),
                  let colon = inner.range(of: ":", range: keyEnd.upperBound..<inner.endIndex),
                  let valStart = inner.range(of: "\"", range: colon.upperBound..<inner.endIndex),
                  let valEnd = inner.range(of: "\"", range: valStart.upperBound..<inner.endIndex)
            else { break }
            out[String(inner[keyStart.upperBound..<keyEnd.lowerBound])] =
                String(inner[valStart.upperBound..<valEnd.lowerBound])
            i = valEnd.upperBound
        }
        return out
    }

    /// 从落盘 JSON 数组恢复 wire 事件字符串（按顶层 {} 配对切分，原样保真）。
    static func parseWireList(_ raw: String) -> [String]? {
        var out: [String] = []
        var depth = 0
        var inString = false
        var escaped = false
        var objStart: String.Index? = nil
        for (index, ch) in raw.enumerated() {
            let idx = raw.index(raw.startIndex, offsetBy: index)
            if inString {
                if escaped {
                    escaped = false
                } else if ch == "\\" {
                    escaped = true
                } else if ch == "\"" {
                    inString = false
                }
                continue
            }
            switch ch {
            case "\"": inString = true
            case "{":
                if depth == 0 { objStart = idx }
                depth += 1
            case "}":
                depth -= 1
                if depth == 0, let start = objStart {
                    out.append(String(raw[start...idx]))
                    objStart = nil
                }
            default: break
            }
        }
        return out
    }
}
