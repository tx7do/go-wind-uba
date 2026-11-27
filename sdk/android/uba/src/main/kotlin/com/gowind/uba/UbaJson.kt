package com.gowind.uba

/**
 * 手写零依赖 JSON 序列化（对齐 Go/Java SDK 的约定）：
 * 按构造顺序输出，跳过 null 与空集合/空 Map，camelCase 由调用方键名保证。
 * Android 与 JVM 通用，不依赖 org.json。
 */
internal object UbaJson {

    fun eventToWire(e: ReportEvent): String {
        val sb = StringBuilder(256)
        sb.append('{')
        sb.append("\"eventType\":").append(str(e.eventType)).append(',')
        sb.append("\"eventId\":").append(str(e.eventId)).append(',')
        sb.append("\"eventName\":").append(str(e.eventName)).append(',')
        sb.append("\"eventTime\":").append(str(e.eventTime))
        put(sb, "userId", e.userId)
        put(sb, "deviceId", e.deviceId)
        put(sb, "sessionId", e.sessionId)
        put(sb, "platform", e.platform)
        put(sb, "eventCategory", e.eventCategory)
        put(sb, "properties", e.properties)
        put(sb, "metrics", e.metrics)
        put(sb, "traceId", e.traceId)
        // 顶层字符串镜像维度；数值标量不落顶层（服务端只信任 oneof 内的值）
        put(sb, "eventAction", e.eventAction)
        put(sb, "objectType", e.objectType)
        put(sb, "objectId", e.objectId)
        put(sb, "objectName", e.objectName)
        put(sb, "amount", e.amount)
        put(sb, "opResult", e.opResult)
        put(sb, "errorCode", e.errorCode)
        e.behavior?.let { sb.append(',').append("\"behavior\":").append(behaviorToWire(it)) }
        e.risk?.let { sb.append(',').append("\"risk\":").append(riskToWire(it)) }
        sb.append('}')
        return sb.toString()
    }

    /** envelope：{appId, appSecret, clientInfo?, events[]}。[events] 为 wire JSON 事件串。 */
    fun envelope(appId: String, appSecret: String, userAgent: String?, events: List<String>): String {
        val sb = StringBuilder(64 + events.sumOf { it.length + 1 })
        sb.append("{\"appId\":").append(jsonStr(appId))
        sb.append(",\"appSecret\":").append(jsonStr(appSecret))
        if (!userAgent.isNullOrEmpty()) {
            sb.append(",\"clientInfo\":{\"userAgent\":").append(jsonStr(userAgent)).append('}')
        }
        sb.append(",\"events\":[")
        events.forEachIndexed { i, e ->
            if (i > 0) sb.append(',')
            sb.append(e)
        }
        sb.append("]}")
        return sb.toString()
    }

    private fun behaviorToWire(b: BehaviorPayload): String {
        val sb = StringBuilder(128)
        sb.append('{')
        var first = true
        // 字符串值走 entryStr（加引号转义）；数值/已序列化 wire 走 entryRaw（原样）
        fun entryRaw(key: String, wire: String?) {
            if (wire.isNullOrEmpty()) return
            if (!first) sb.append(',')
            first = false
            sb.append(str(key)).append(':').append(wire)
        }
        fun entryStr(key: String, value: String?) = entryRaw(key, value?.let { str(it) })
        entryStr("eventAction", b.eventAction)
        entryStr("objectType", b.objectType)
        entryStr("objectId", b.objectId)
        entryStr("objectName", b.objectName)
        entryRaw("sessionSeq", b.sessionSeq?.toString())
        entryStr("os", b.os)
        entryStr("appVersion", b.appVersion)
        entryStr("channel", b.channel)
        entryStr("network", b.network)
        entryRaw("durationMs", b.durationMs?.toString())
        entryStr("amount", b.amount)
        entryRaw("quantity", b.quantity?.toString())
        entryRaw("score", b.score?.toString())
        entryRaw("metrics", mapToWire(b.metrics))
        entryRaw("properties", strMapToWire(b.properties))
        entryStr("opResult", b.opResult)
        entryStr("errorCode", b.errorCode)
        entryRaw("clickX", b.clickX?.toString())
        entryRaw("clickY", b.clickY?.toString())
        entryStr("elementXpath", b.elementXpath)
        entryStr("pageUrl", b.pageUrl)
        entryRaw("viewportWidth", b.viewportWidth?.toString())
        entryStr("serverId", b.serverId)
        entryRaw("level", b.level?.toString())
        sb.append('}')
        return sb.toString()
    }

    private fun riskToWire(r: RiskEvent): String {
        val sb = StringBuilder(128)
        sb.append('{')
        var first = true
        // 字符串值走 entryStr（加引号转义）；数值/已序列化 wire 走 entryRaw（原样）
        fun entryRaw(key: String, wire: String?) {
            if (wire.isNullOrEmpty()) return
            if (!first) sb.append(',')
            first = false
            sb.append(str(key)).append(':').append(wire)
        }
        fun entryStr(key: String, value: String?) = entryRaw(key, value?.let { str(it) })
        entryStr("riskType", r.riskType)
        entryStr("riskLevel", r.riskLevel)
        entryRaw("riskScore", r.riskScore?.toString())
        entryRaw("ruleId", r.ruleId?.toString())
        entryStr("ruleName", r.ruleName)
        if (!r.ruleContext.isNullOrEmpty()) entryRaw("ruleContext", anyMapToWire(r.ruleContext))
        if (!r.relatedEventIds.isNullOrEmpty()) {
            sbReady(sb, first)
            first = false
            sbReady(sb, first); first = false; sb.append(str("relatedEventIds")).append(":[").append(
                r.relatedEventIds.joinToString(",") { str(it) }
            ).append(']')
        }
        entryStr("description", r.description)
        entryStr("evidence", strMapToWire(r.evidence))
        entryStr("status", r.status)
        sb.append('}')
        return sb.toString()
    }

    private fun sbReady(sb: StringBuilder, first: Boolean) {
        if (!first) sb.append(',')
    }

    private fun strMapToWire(map: Map<String, String>?): String? {
        if (map.isNullOrEmpty()) return null
        return map.entries.joinToString(",", prefix = "{", postfix = "}") { (k, v) ->
            "${str(k)}:${str(v)}"
        }
    }

    private fun mapToWire(map: Map<String, Double>?): String? {
        if (map.isNullOrEmpty()) return null
        return map.entries.joinToString(",", prefix = "{", postfix = "}") { (k, v) ->
            "${str(k)}:$v"
        }
    }

    private fun anyMapToWire(map: Map<String, Any?>): String {
        return map.entries.joinToString(",", prefix = "{", postfix = "}") { (k, v) ->
            val wire = when (v) {
                null -> null
                is String -> str(v)
                is Number -> v.toString()
                is Boolean -> v.toString()
                is Map<*, *> -> {
                    @Suppress("UNCHECKED_CAST")
                    anyMapToWire(v as Map<String, Any?>)
                }
                else -> str(v.toString())
            }
            "${str(k)}:${wire ?: "null"}"
        }
    }

    private fun put(sb: StringBuilder, key: String, value: String?) {
        if (value.isNullOrEmpty()) return
        sb.append(',').append(str(key)).append(':').append(str(value))
    }

    private fun put(sb: StringBuilder, key: String, value: Long?) {
        if (value == null) return
        sb.append(',').append(str(key)).append(':').append(value)
    }

    private fun put(sb: StringBuilder, key: String, value: Map<String, String>?) {
        val wire = strMapToWire(value) ?: return
        sb.append(',').append(str(key)).append(':').append(wire)
    }

    // Map<String,String> 与 Map<String,Double> 泛型擦除后 JVM 签名相同，@JvmName 防冲突
    @JvmName("putDoubleMap")
    private fun put(sb: StringBuilder, key: String, value: Map<String, Double>?) {
        val wire = mapToWire(value) ?: return
        sb.append(',').append(str(key)).append(':').append(wire)
    }

    private fun str(s: String): String = jsonStr(s)
}

/** JSON 字符串字面量（含引号与转义）。 */
internal fun jsonStr(s: String): String {
    val sb = StringBuilder(s.length + 2)
    sb.append('"')
    for (c in s) {
        when (c) {
            '"' -> sb.append("\\\"")
            '\\' -> sb.append("\\\\")
            '\n' -> sb.append("\\n")
            '\r' -> sb.append("\\r")
            '\t' -> sb.append("\\t")
            '\b' -> sb.append("\\b")
            '\u000C' -> sb.append("\\f")
            else -> {
                if (c < ' ') sb.append("\\u%04x".format(c.code)) else sb.append(c)
            }
        }
    }
    sb.append('"')
    return sb.toString()
}
