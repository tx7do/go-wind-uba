package com.gowind.uba

import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean

/**
 * GoWind UBA Android 埋点客户端。线程安全。
 *
 * Android 宿主推荐通过 `android/` 目录的适配层接入（SharedPreferences 持久化 +
 * Application.ActivityLifecycleCallbacks 自动接前后台）；纯 JVM/测试环境直接构造：
 *
 * ```
 * val client = UbaClient(
 *     UbaConfig(appId = "your_app_id", appSecret = "your_app_secret",
 *               endpoint = "http://collector:5700"),
 *     store = InMemoryStore(),
 * )
 * client.identify(1001)
 * client.track("button_click", properties = mapOf("scene" to "home"))
 * ```
 *
 * 契约铁律（与 Web/C#/Flutter/Go/Java SDK 一致）：camelCase 对齐 proto json_name；
 * tenantId/serverTime/ip 服务端补全不发送；数值标量只进 behavior oneof；
 * 4xx（除 429）不重试；可重试失败落盘补发（cap maxPersistedEvents）。
 *
 * 离线补发实现：落盘的是事件 wire JSON（发送体原样），恢复后直接重发，无损。
 */
class UbaClient(
    val config: UbaConfig,
    store: KeyValueStore,
    transport: HttpTransport = HttpTransport.UrlConnectionTransport(),
) {
    private val lock = Any()
    private val queue = ArrayDeque<ReportEvent>()

    /** 恢复自离线盘的 wire JSON 事件（原样重发，最旧优先）。 */
    private val restoredWire = ArrayDeque<String>()

    private var flushing = false
    private val closed = AtomicBoolean(false)
    private val store: KeyValueStore = store
    private val transport: HttpTransport = transport

    private val session = SessionManager(config.sessionTimeoutMs)
    private var consentGranted = false
    private val superProps = mutableMapOf<String, String>()
    private val flags = mutableMapOf<String, String>()
    private val exposedFlags = mutableSetOf<String>()
    private var exposedSessionId: String? = null
    private var userId: Long? = null
    private var lastScreenAt: Long? = null

    private val deviceId: String
    private val deviceIdIsNew: Boolean
    private val userAgent = "uba-android/$SDK_VERSION (android)"

    private val timer = if (config.flushIntervalMs > 0) {
        Executors.newSingleThreadScheduledExecutor { r ->
            Thread(r, "uba-flush").apply { isDaemon = true }
        }
    } else null

    init {
        val persisted = store.getString(DEVICE_ID_KEY)
        deviceIdIsNew = persisted.isNullOrEmpty()
        deviceId = if (deviceIdIsNew) uuidV4() else persisted
        if (deviceIdIsNew) store.setString(DEVICE_ID_KEY, deviceId)

        restorePersisted()
        timer?.scheduleWithFixedDelay({
            if (synchronized(lock) { queue.isNotEmpty() || restoredWire.isNotEmpty() } && !closed.get()) {
                runCatching { flush() }
            }
        }, config.flushIntervalMs, config.flushIntervalMs, TimeUnit.MILLISECONDS)

        if (config.autoTrackLaunch && deviceIdIsNew) {
            track(
                "app_launch",
                properties = mapOf("first_launch" to "true"),
                options = TrackOptions(eventCategory = "app"),
            )
        }
    }

    // ─── 上报 API ────────────────────────────────────────────────────────────

    /** 上报自定义行为事件。 */
    @JvmOverloads
    fun track(
        eventName: String,
        properties: Map<String, String>? = null,
        options: TrackOptions? = null,
    ) = trackBehavior(eventName, properties, options)

    /** 上报自定义行为事件。可重试失败留在内存队列等待补发。 */
    fun trackBehavior(
        eventName: String,
        properties: Map<String, String>? = null,
        options: TrackOptions? = null,
    ) {
        if (options?.sessionId == null) session.checkTimeout() // 事件间隔超时轮换
        val event = buildBehaviorEvent(eventName, properties, options) ?: return
        enqueue(event)
    }

    /** 上报风险事件（进入 uba_risk_events 通道）。 */
    fun trackRisk(
        eventName: String,
        risk: RiskEvent,
        properties: Map<String, String>? = null,
        options: TrackOptions? = null,
    ) {
        if (options?.sessionId == null) session.checkTimeout() // 事件间隔超时轮换
        if (!shouldCollect(eventName)) return
        enqueue(
            ReportEvent(
                eventType = "RISK",
                eventId = uuidV4(),
                eventName = eventName,
                eventTime = toRFC3339Ms(System.currentTimeMillis()),
                userId = options?.userId ?: userId,
                deviceId = options?.deviceId ?: deviceId,
                sessionId = options?.sessionId ?: session.current,
                platform = options?.platform ?: PLATFORM_ANDROID,
                eventCategory = options?.eventCategory,
                properties = mergedProperties(properties, options),
                metrics = options?.metrics,
                traceId = options?.traceId,
                eventAction = null,
                objectType = null,
                objectId = null,
                objectName = null,
                amount = null,
                opResult = null,
                errorCode = null,
                behavior = null,
                risk = risk,
            ),
        )
    }

    /** 页面浏览事件：自动携带上一页面停留时长 durationMs。 */
    @JvmOverloads
    fun trackPageView(screenName: String, title: String? = null, referer: String? = null) {
        val now = System.currentTimeMillis()
        val dwell = lastScreenAt?.let { now - it }
        lastScreenAt = now
        val props = mutableMapOf<String, String>("pageUrl" to screenName)
        if (!title.isNullOrEmpty()) props["title"] = title
        if (!referer.isNullOrEmpty()) props["referer"] = referer
        trackBehavior(
            config.pageViewEvent,
            props,
            TrackOptions(
                eventCategory = "navigation",
                objectType = "page",
                objectId = screenName,
                durationMs = dwell?.takeIf { it > 0 },
            ),
        )
    }

    /** 登录绑定用户 ID。 */
    fun identify(id: Long) {
        userId = id
    }

    /** 登出清空用户绑定。 */
    fun resetUser() {
        userId = null
    }

    /** 设置全局公共属性：合并进之后每个事件（同名覆盖）。 */
    fun setSuperProperties(props: Map<String, String>) {
        synchronized(lock) { superProps.putAll(props) }
    }

    /** 清空全局公共属性。 */
    fun clearSuperProperties() {
        synchronized(lock) { superProps.clear() }
    }

    /** 隐私合规授权开关；requireConsent = true 时未授权前所有上报静默丢弃。 */
    fun setConsent(granted: Boolean) {
        consentGranted = granted
    }

    /** 当前是否处于采集状态（未关闭、consent 放行、会话命中采样）。 */
    fun isCollecting(): Boolean = !closed.get() && consentOk() && session.inSample(config.sampleRate)

    // ─── 生命周期（适配层自动调用） ────────────────────────────────────────────

    /** 退到后台：立即 flush + 记录时刻。 */
    fun handleBackground() {
        session.onBackground()
        runCatching { flush() }
    }

    /** 回到前台：后台超过 sessionTimeoutMs 则轮换新会话。 */
    fun handleForeground() {
        session.onForeground()
    }

    // ─── AB 实验开关 ──────────────────────────────────────────────────────────

    /** 拉取最新实验开关并缓存。失败保留旧缓存并返回当前缓存。 */
    fun loadFlags(): Map<String, String> {
        val body = buildString {
            append("{\"appId\":").append(jsonStr(config.appId))
            append(",\"appSecret\":").append(jsonStr(config.appSecret))
            append(",\"deviceId\":").append(jsonStr(deviceId))
            userId?.let { append(",\"userId\":").append(it) }
            append('}')
        }

        val res = transport.send(config.flagsUrl, body, config.timeoutMs)
        if (res.isOk) {
            val parsed = parseFlags(res.body)
            synchronized(lock) {
                flags.clear()
                flags.putAll(parsed)
            }
        } else {
            debugLog("拉取 flags 失败：HTTP ${res.statusCode}")
        }
        return synchronized(lock) { flags.toMap() }
    }

    /** 读取开关变体；命中时自动上报一次 $feature_flag 曝光（每会话每开关一次）。 */
    fun getFlag(key: String): String? {
        val variant = synchronized(lock) { flags[key] } ?: return null
        trackFlagExposure(key, variant)
        return variant
    }

    /** 变体非空且非 control 即视为命中（不触发曝光事件）。 */
    fun isFeatureEnabled(key: String): Boolean {
        val v = synchronized(lock) { flags[key] } ?: return false
        return v.isNotEmpty() && v != "control"
    }

    // ─── 维护 ────────────────────────────────────────────────────────────────

    /** 队列中待上报条数（含离线恢复的）。 */
    fun pendingCount(): Int = synchronized(lock) { queue.size + restoredWire.size }

    /** 手动冲刷队列：分批发送直到清空或遇到失败。并发调用安全。 */
    fun flush(): FlushResult {
        if (closed.get()) return FlushResult(success = true)
        synchronized(lock) {
            if (flushing) return FlushResult(success = true)
            flushing = true
        }
        var sent = 0
        var dropped = 0
        var ok = true
        var retryable = false
        var message: String? = null
        try {
            // 1. 先补发离线恢复的 wire 事件
            while (true) {
                val batch = synchronized(lock) {
                    if (restoredWire.isEmpty()) null else takeWireBatchLocked()
                } ?: break
                val r = sendWireWithRetry(batch)
                sent += r.sent
                dropped += r.dropped
                if (!r.success) {
                    ok = false
                    message = r.message
                    retryable = r.retryable
                    if (retryable) synchronized(lock) { batch.asReversed().forEach { restoredWire.addFirst(it) } }
                    break
                }
            }
            // 2. 再发内存队列的新事件
            if (ok) {
                while (true) {
                    val batch = synchronized(lock) {
                        if (queue.isEmpty()) null else takeBatchLocked()
                    } ?: break
                    val r = sendWithRetry(batch)
                    sent += r.sent
                    dropped += r.dropped
                    if (!r.success) {
                        ok = false
                        message = r.message
                        retryable = r.retryable
                        if (retryable) synchronized(lock) { batch.asReversed().forEach { queue.addFirst(it) } }
                        break
                    }
                }
            }
            persist()
        } finally {
            synchronized(lock) { flushing = false }
        }
        return FlushResult(ok, sent, dropped, retryable, message)
    }

    /** 关闭：停止定时器并冲刷剩余队列。之后 track 不再入队。 */
    fun close(): FlushResult {
        timer?.shutdownNow()
        val result = flush()
        closed.set(true)
        transport.close()
        return result
    }

    // ─── 内部实现 ────────────────────────────────────────────────────────────

    private fun consentOk(): Boolean = !config.requireConsent || consentGranted

    private fun shouldCollect(eventName: String): Boolean {
        if (closed.get()) return false
        if (!consentOk()) {
            debugLog("consent 未授权，丢弃事件 $eventName")
            return false
        }
        return session.inSample(config.sampleRate)
    }

    private fun buildBehaviorEvent(
        eventName: String,
        properties: Map<String, String>?,
        options: TrackOptions?,
    ): ReportEvent? {
        if (!shouldCollect(eventName)) return null
        val o = options
        // 数值标量只进 behavior oneof；顶层只镜像字符串维度
        val behavior = BehaviorPayload(
            eventAction = o?.eventAction,
            objectType = o?.objectType,
            objectId = o?.objectId,
            objectName = o?.objectName,
            os = "Android",
            appVersion = config.appVersion,
            channel = config.channel,
            network = o?.network,
            durationMs = o?.durationMs,
            amount = o?.amount,
            quantity = o?.quantity,
            score = o?.score,
            metrics = o?.metrics,
            opResult = o?.opResult,
            errorCode = o?.errorCode,
            serverId = o?.serverId,
            level = o?.level,
        )
        return ReportEvent(
            eventType = "BEHAVIOR",
            eventId = uuidV4(),
            eventName = eventName,
            eventTime = toRFC3339Ms(System.currentTimeMillis()),
            userId = o?.userId ?: userId,
            deviceId = o?.deviceId ?: deviceId,
            sessionId = o?.sessionId ?: session.current,
            platform = o?.platform ?: PLATFORM_ANDROID,
            eventCategory = o?.eventCategory,
            properties = mergedProperties(properties, o),
            metrics = o?.metrics,
            traceId = o?.traceId,
            eventAction = o?.eventAction,
            objectType = o?.objectType,
            objectId = o?.objectId,
            objectName = o?.objectName,
            amount = o?.amount,
            opResult = o?.opResult,
            errorCode = o?.errorCode,
            behavior = behavior,
            risk = null,
        )
    }

    private fun mergedProperties(
        properties: Map<String, String>?,
        options: TrackOptions?,
    ): Map<String, String>? {
        val merged = synchronized(lock) { superProps.toMutableMap() }
        properties?.let { merged.putAll(it) }
        options?.properties?.let { merged.putAll(it) }
        return merged.ifEmpty { null }
    }

    private fun enqueue(event: ReportEvent) {
        var autoFlush = false
        synchronized(lock) {
            queue.addLast(event)
            if (queue.size >= config.batchSize) autoFlush = true
        }
        if (autoFlush) {
            Thread {
                runCatching { flush() }
            }.apply {
                isDaemon = true
                name = "uba-auto-flush"
                start()
            }
        }
    }

    /** 取一批 wire 事件：受 batchSize 与 maxBatchBytes 双重限制。调用方需持锁。 */
    private fun takeWireBatchLocked(): List<String> {
        val batch = mutableListOf<String>()
        var bytes = 0
        while (restoredWire.isNotEmpty() && batch.size < config.batchSize) {
            val next = restoredWire.first()
            if (batch.isNotEmpty() && bytes + next.length > config.maxBatchBytes) break
            bytes += next.length
            batch.add(restoredWire.removeFirst())
        }
        return batch
    }

    /** 取一批新事件：受 batchSize 与 maxBatchBytes 双重限制。调用方需持锁。 */
    private fun takeBatchLocked(): List<ReportEvent> {
        val batch = mutableListOf<ReportEvent>()
        var bytes = 0
        while (queue.isNotEmpty() && batch.size < config.batchSize) {
            val encoded = UbaJson.eventToWire(queue.first()).length
            if (batch.isNotEmpty() && bytes + encoded > config.maxBatchBytes) break
            bytes += encoded
            batch.add(queue.removeFirst())
        }
        return batch
    }

    private fun sendWithRetry(batch: List<ReportEvent>): FlushResult =
        sendWireWithRetry(batch.map { UbaJson.eventToWire(it) })

    private fun sendWireWithRetry(batch: List<String>): FlushResult {
        val body = UbaJson.envelope(config.appId, config.appSecret, userAgent, batch)
        for (attempt in 0..config.maxRetries) {
            val res = transport.send(config.reportUrl, body, config.timeoutMs)

            if (res.isOk) {
                val failed = parseFailedCount(res.body)
                if (failed > 0) debugLog("部分事件被服务端拒绝：HTTP 200 failedCount=$failed")
                return FlushResult(true, batch.size - failed, failed)
            }

            // 4xx（除 429）是确定性失败：整批丢弃，不重试
            if (res.statusCode in 400..499 && res.statusCode != 429) {
                debugLog("HTTP ${res.statusCode}（不可重试），丢弃 ${batch.size} 条")
                return FlushResult(false, 0, batch.size, retryable = false, message = "HTTP ${res.statusCode}")
            }

            if (attempt >= config.maxRetries) {
                debugLog("重试耗尽（HTTP ${res.statusCode}），${batch.size} 条待落盘补发")
                return FlushResult(
                    false, 0, batch.size, retryable = true,
                    message = "HTTP ${res.statusCode} after ${attempt + 1} attempts",
                )
            }

            val delay = backoffDelayMs(config.retryBaseDelayMs, attempt)
            debugLog("HTTP ${res.statusCode}，${delay}ms 后第 ${attempt + 1} 次重试")
            try {
                Thread.sleep(delay)
            } catch (e: InterruptedException) {
                Thread.currentThread().interrupt()
                return FlushResult(false, 0, batch.size, retryable = true, message = "interrupted")
            }
        }
        return FlushResult(false, 0, batch.size, retryable = true, message = "unreachable")
    }

    private fun restorePersisted() {
        val raw = store.getString(OFFLINE_QUEUE_KEY) ?: return
        if (raw.isEmpty()) return
        val events = runCatching { parseWireList(raw) }.getOrNull()
        if (events == null) {
            debugLog("离线队列解析失败，已忽略")
            store.remove(OFFLINE_QUEUE_KEY)
            return
        }
        synchronized(lock) { events.asReversed().forEach { restoredWire.addFirst(it) } }
        store.remove(OFFLINE_QUEUE_KEY)
        debugLog("离线队列恢复 ${events.size} 条")
    }

    private fun persist() {
        val snapshot = synchronized(lock) {
            restoredWire.toList() + queue.map { UbaJson.eventToWire(it) }
        }
        if (snapshot.isEmpty()) {
            store.remove(OFFLINE_QUEUE_KEY)
            return
        }
        val capped = snapshot.takeLast(config.maxPersistedEvents)
        store.setString(OFFLINE_QUEUE_KEY, "[" + capped.joinToString(",") + "]")
    }

    private fun trackFlagExposure(key: String, variant: String) {
        val sessionId = session.current
        val first = synchronized(lock) {
            if (exposedSessionId != sessionId) {
                exposedFlags.clear()
                exposedSessionId = sessionId
            }
            exposedFlags.add(key)
        }
        if (first) {
            track(
                "\$feature_flag",
                mapOf("flag_key" to key, "variant" to variant),
                TrackOptions(eventCategory = "experiment"),
            )
        }
    }

    private fun debugLog(message: String) {
        if (!config.debug) return
        println("[uba-android] $message")
    }
}

// ─── 轻量 JSON 解析（flags / failedCount / 离线队列结构，避免引序列化库） ──────────

internal fun parseFailedCount(body: String): Int {
    val idx = body.indexOf("\"failedCount\"")
    if (idx < 0) return 0
    val colon = body.indexOf(':', idx)
    if (colon < 0) return 0
    return body.drop(colon + 1).takeWhile { it.isDigit() }.toIntOrNull() ?: 0
}

internal fun parseFlags(body: String): Map<String, String> {
    val flagsKey = body.indexOf("\"flags\"")
    if (flagsKey < 0) return emptyMap()
    val objStart = body.indexOf('{', flagsKey)
    if (objStart < 0) return emptyMap()
    var depth = 0
    var end = -1
    for (i in objStart until body.length) {
        when (body[i]) {
            '{' -> depth++
            '}' -> {
                depth--
                if (depth == 0) {
                    end = i
                    break
                }
            }
        }
    }
    if (end < 0) return emptyMap()
    val inner = body.substring(objStart + 1, end)
    val out = mutableMapOf<String, String>()
    var i = 0
    while (i < inner.length) {
        val keyStart = inner.indexOf('"', i)
        if (keyStart < 0) break
        val keyEnd = inner.indexOf('"', keyStart + 1)
        if (keyEnd < 0) break
        val colon = inner.indexOf(':', keyEnd + 1)
        if (colon < 0) break
        val valStart = inner.indexOf('"', colon + 1)
        if (valStart < 0) break
        val valEnd = inner.indexOf('"', valStart + 1)
        if (valEnd < 0) break
        out[inner.substring(keyStart + 1, keyEnd)] = inner.substring(valStart + 1, valEnd)
        i = valEnd + 1
    }
    return out
}

/** 从落盘 JSON 数组恢复 wire 事件字符串（按顶层 `{}` 配对切分，原样保真）。 */
internal fun parseWireList(raw: String): List<String> {
    val out = mutableListOf<String>()
    var depth = 0
    var inString = false
    var escaped = false
    var objStart = -1
    for ((index, ch) in raw.withIndex()) {
        if (inString) {
            when (ch) {
                '\\' -> escaped = !escaped
                '"' -> if (!escaped) inString = false
            }
            if (ch != '\\') escaped = false
            continue
        }
        when (ch) {
            '"' -> inString = true
            '{' -> {
                if (depth == 0) objStart = index
                depth++
            }
            '}' -> {
                depth--
                if (depth == 0 && objStart >= 0) {
                    out.add(raw.substring(objStart, index + 1))
                    objStart = -1
                }
            }
        }
    }
    return out
}
