package com.gowind.uba

import java.time.ZoneOffset
import java.time.format.DateTimeFormatter
import java.util.UUID

/** RFC3339 毫秒 UTC（与其他端 SDK 精度一致）。 */
private val RFC3339_MS: DateTimeFormatter =
    DateTimeFormatter.ofPattern("yyyy-MM-dd'T'HH:mm:ss.SSS'Z'").withZone(ZoneOffset.UTC)

internal fun toRFC3339Ms(epochMillis: Long): String =
    RFC3339_MS.format(java.time.Instant.ofEpochMilli(epochMillis))

internal fun uuidV4(): String = UUID.randomUUID().toString()

/** FNV-1a 32 位哈希：按 sessionId 确定性采样（与 Web/Flutter/Go SDK 算法一致）。 */
internal fun fnv1a32(s: String): Long {
    var hash = 0x811c9dc5L
    for (ch in s) {
        hash = hash xor (ch.code and 0xff).toLong()
        hash *= 0x01000193L
        hash = hash and 0xffffffffL
    }
    return hash
}

/** 会话是否落在采样区间：hash(sessionId) % 1000 / 1000 < sampleRate。 */
internal fun sessionIdInSample(sessionId: String, sampleRate: Double): Boolean {
    if (sampleRate >= 1) return true
    if (sampleRate <= 0) return false
    return (fnv1a32(sessionId) % 1000) / 1000.0 < sampleRate
}

/**
 * 会话管理：进程内 uuid，后台超过 [timeoutMs] 后回到前台时轮换新会话。
 * [nowProvider] 注入可测时钟。
 */
class SessionManager(
    private val timeoutMs: Long = 30000,
    private val nowProvider: () -> Long = { System.currentTimeMillis() },
) {
    private var id: String = uuidV4()
    private var pausedAt: Long? = null
    private var lastEventAt: Long? = null

    val current: String get() = id

    fun inSample(sampleRate: Double): Boolean = sessionIdInSample(id, sampleRate)

    /** 进入后台（onPause/onStop）。 */
    fun onBackground() {
        if (pausedAt == null) pausedAt = nowProvider()
    }

    /** 回到前台（onResume）：后台超过 timeoutMs 则轮换新会话。 */
    fun onForeground() {
        val paused = pausedAt
        pausedAt = null
        if (paused != null && timeoutMs > 0 && nowProvider() - paused > timeoutMs) {
            rotate()
        }
    }

    /** 事件间隔超时轮换（与 Go/Java SDK 的 currentSessionID 语义一致）：
     *  距上次事件超过 timeoutMs 则开启新会话。返回当前（可能已轮换的）会话 ID。 */
    fun checkTimeout(): String {
        val now = nowProvider()
        lastEventAt?.let { last ->
            if (timeoutMs > 0 && now - last > timeoutMs) rotate()
        }
        lastEventAt = now
        return id
    }

    /** 手动开启新会话。 */
    fun rotate() {
        id = uuidV4()
        pausedAt = null
    }
}
