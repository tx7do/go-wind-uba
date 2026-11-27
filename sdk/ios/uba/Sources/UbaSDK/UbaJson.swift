import Foundation

/// 手写零依赖 JSON 序列化（对齐 Go/Java/Kotlin SDK 的约定）：
/// 按构造顺序输出，跳过 null 与空集合，camelCase 由调用方键名保证。
enum UbaJson {

    static func eventToWire(_ e: ReportEvent) -> String {
        var parts: [String] = []
        parts.append("\"eventType\":\(str(e.eventType))")
        parts.append("\"eventId\":\(str(e.eventId))")
        parts.append("\"eventName\":\(str(e.eventName))")
        parts.append("\"eventTime\":\(str(e.eventTime))")
        put(&parts, "userId", e.userId)
        put(&parts, "deviceId", e.deviceId)
        put(&parts, "sessionId", e.sessionId)
        put(&parts, "platform", e.platform)
        put(&parts, "eventCategory", e.eventCategory)
        put(&parts, "properties", dictStr(e.properties))
        put(&parts, "metrics", dictDouble(e.metrics))
        put(&parts, "traceId", e.traceId)
        // 顶层字符串镜像维度；数值标量不落顶层（服务端只信任 oneof 内的值）
        put(&parts, "eventAction", e.eventAction)
        put(&parts, "objectType", e.objectType)
        put(&parts, "objectId", e.objectId)
        put(&parts, "objectName", e.objectName)
        put(&parts, "amount", e.amount)
        put(&parts, "opResult", e.opResult)
        put(&parts, "errorCode", e.errorCode)
        if let b = e.behavior {
            parts.append("\"behavior\":\(behaviorToWire(b))")
        }
        if let r = e.risk {
            parts.append("\"risk\":\(riskToWire(r))")
        }
        return "{" + parts.joined(separator: ",") + "}"
    }

    /// envelope：{appId, appSecret, clientInfo?, events[]}。events 为 wire JSON 事件串。
    static func envelope(_ appId: String, _ appSecret: String, _ userAgent: String?,
                         _ events: [String]) -> String {
        var parts: [String] = [
            "\"appId\":\(str(appId))",
            "\"appSecret\":\(str(appSecret))",
        ]
        if let ua = userAgent, !ua.isEmpty {
            parts.append("\"clientInfo\":{\"userAgent\":\(str(ua))}")
        }
        parts.append("\"events\":[\(events.joined(separator: ","))]")
        return "{" + parts.joined(separator: ",") + "}"
    }

    // MARK: 载荷

    private static func behaviorToWire(_ b: BehaviorPayload) -> String {
        var parts: [String] = []
        put(&parts, "eventAction", b.eventAction)
        put(&parts, "objectType", b.objectType)
        put(&parts, "objectId", b.objectId)
        put(&parts, "objectName", b.objectName)
        put(&parts, "sessionSeq", b.sessionSeq)
        put(&parts, "os", b.os)
        put(&parts, "appVersion", b.appVersion)
        put(&parts, "channel", b.channel)
        put(&parts, "network", b.network)
        put(&parts, "durationMs", b.durationMs)
        put(&parts, "amount", b.amount)
        put(&parts, "quantity", b.quantity)
        put(&parts, "score", b.score.map { Int64($0) })
        put(&parts, "metrics", dictDouble(b.metrics))
        put(&parts, "opResult", b.opResult)
        put(&parts, "errorCode", b.errorCode)
        put(&parts, "serverId", b.serverId)
        put(&parts, "level", b.level.map { Int64($0) })
        return "{" + parts.joined(separator: ",") + "}"
    }

    private static func riskToWire(_ r: RiskEvent) -> String {
        var parts: [String] = []
        put(&parts, "riskType", r.riskType)
        put(&parts, "riskLevel", r.riskLevel)
        put(&parts, "riskScore", r.riskScore)
        put(&parts, "ruleId", r.ruleId)
        put(&parts, "ruleName", r.ruleName)
        if let ctx = r.ruleContext, !ctx.isEmpty {
            parts.append("\"ruleContext\":\(anyJSON(ctx))")
        }
        if let ids = r.relatedEventIds, !ids.isEmpty {
            let items = ids.map { str($0) }.joined(separator: ",")
            parts.append("\"relatedEventIds\":[\(items)]")
        }
        put(&parts, "description", r.description)
        put(&parts, "evidence", dictStr(r.evidence))
        put(&parts, "status", r.status)
        return "{" + parts.joined(separator: ",") + "}"
    }

    // MARK: 值与集合

    private static func put(_ parts: inout [String], _ key: String, _ value: String?) {
        guard let v = value, !v.isEmpty else { return }
        parts.append("\"\(key)\":\(str(v))")
    }

    private static func put(_ parts: inout [String], _ key: String, _ value: Int64?) {
        guard let v = value else { return }
        parts.append("\"\(key)\":\(v)")
    }

    private static func put(_ parts: inout [String], _ key: String, _ value: Double?) {
        guard let v = value else { return }
        parts.append("\"\(key)\":\(numString(v))")
    }

    private static func put(_ parts: inout [String], _ key: String, _ dict: String?) {
        guard let v = dict else { return }
        parts.append("\"\(key)\":\(v)")
    }

    private static func dictStr(_ dict: [String: String]?) -> String? {
        guard let d = dict, !d.isEmpty else { return nil }
        let items = d.map { "\(str($0.key)):\(str($0.value))" }.sorted()
        return "{" + items.joined(separator: ",") + "}"
    }

    private static func dictDouble(_ dict: [String: Double]?) -> String? {
        guard let d = dict, !d.isEmpty else { return nil }
        let items = d.map { "\(str($0.key)):\(numString($0.value))" }.sorted()
        return "{" + items.joined(separator: ",") + "}"
    }

    /// 任意 JSON 对象（ruleContext 等）：支持 String/Number/Bool/嵌套 Map/Array。
    private static func anyJSON(_ value: Any) -> String {
        switch value {
        case let s as String: return str(s)
        case let n as NSNumber:
            // Bool 在 Foundation 里是 NSNumber 子类型，需先判
            if CFGetTypeID(n) == CFBooleanGetTypeID() {
                return n.boolValue ? "true" : "false"
            }
            return numString(n.doubleValue)
        case let b as Bool: return b ? "true" : "false"
        case let d as Double: return numString(d)
        case let i as Int: return String(i)
        case let i64 as Int64: return String(i64)
        case let dict as [String: Any]:
            let items = dict.map { "\(str($0.key)):\(anyJSON($0.value))" }.sorted()
            return "{" + items.joined(separator: ",") + "}"
        case let array as [Any]:
            return "[" + array.map(anyJSON).joined(separator: ",") + "]"
        default:
            return str(String(describing: value))
        }
    }

    /// Double 输出：整数值不带小数点（对齐其他端 SDK 的行为）。
    private static func numString(_ d: Double) -> String {
        if d == d.rounded() && abs(d) < 1e15 && !d.isInfinite {
            return String(Int64(d))
        }
        return String(d)
    }

    /// JSON 字符串字面量（含引号与转义）。
    static func str(_ s: String) -> String {
        var out = "\""
        for scalar in s.unicodeScalars {
            switch scalar {
            case "\"": out += "\\\""
            case "\\": out += "\\\\"
            case "\n": out += "\\n"
            case "\r": out += "\\r"
            case "\t": out += "\\t"
            default:
                if scalar.value < 0x20 {
                    out += String(format: "\\u%04x", scalar.value)
                } else {
                    out.unicodeScalars.append(scalar)
                }
            }
        }
        out += "\""
        return out
    }
}
