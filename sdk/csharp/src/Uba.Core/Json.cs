using System.Collections.Generic;
using System.Globalization;
using System.Text;

namespace Uba
{
    /// <summary>
    /// 零依赖 JSON 序列化（手写）。
    /// 仅服务于本库的请求序列化（camelCase 键、跳过 null）与响应反序列化（关键字段）。
    /// 不实现通用 JSON，保持极简、可审计。
    /// </summary>
    internal static class JsonSerializer
    {
        // ──────────── 序列化（请求体）────────────

        public static string Serialize(PostReportRequest req)
        {
            var sb = new StringBuilder(1024);
            sb.Append('{');
            WriteString(sb, "appId", req.AppId); sb.Append(',');
            WriteString(sb, "appSecret", req.AppSecret); sb.Append(',');
            // clientInfo
            if (req.ClientInfo != null)
            {
                sb.Append("\"clientInfo\":");
                AppendClientInfo(sb, req.ClientInfo);
                sb.Append(',');
            }
            // events
            sb.Append("\"events\":[");
            for (int i = 0; i < req.Events.Count; i++)
            {
                if (i > 0) sb.Append(',');
                AppendReportEvent(sb, req.Events[i]);
            }
            sb.Append(']');
            sb.Append('}');
            return sb.ToString();
        }

        private static void AppendClientInfo(StringBuilder sb, ClientInfo ci)
        {
            sb.Append('{');
            bool first = true;
            first = WriteStringIfNotNull(sb, "userAgent", ci.UserAgent, first);
            first = WriteStringIfNotNull(sb, "referer", ci.Referer, first);
            first = WriteStringIfNotNull(sb, "country", ci.Country, first);
            WriteStringIfNotNull(sb, "city", ci.City, first);
            sb.Append('}');
        }

        private static void AppendReportEvent(StringBuilder sb, ReportEvent e)
        {
            sb.Append('{');
            bool first = true;
            first = WriteStringIfNotNull(sb, "eventType", e.EventType, first);
            first = WriteStringIfNotNull(sb, "eventId", e.EventId, first);
            if (e.UserId.HasValue) { Sep(sb, ref first); sb.Append("\"userId\":").Append(e.UserId.Value); }
            first = WriteStringIfNotNull(sb, "deviceId", e.DeviceId, first);
            first = WriteStringIfNotNull(sb, "eventTime", e.EventTime, first);
            first = WriteStringIfNotNull(sb, "eventName", e.EventName, first);
            first = WriteStringIfNotNull(sb, "eventCategory", e.EventCategory, first);
            first = WriteStringIfNotNull(sb, "sessionId", e.SessionId, first);
            first = WriteStringIfNotNull(sb, "platform", e.Platform, first);
            first = WriteStringIfNotNull(sb, "ip", e.Ip, first);
            first = WriteStringMap(sb, "properties", e.Properties, first);
            first = WriteStringIfNotNull(sb, "traceId", e.TraceId, first);
            first = WriteStringIfNotNull(sb, "eventAction", e.EventAction, first);
            first = WriteStringIfNotNull(sb, "objectType", e.ObjectType, first);
            first = WriteStringIfNotNull(sb, "objectId", e.ObjectId, first);
            first = WriteStringIfNotNull(sb, "objectName", e.ObjectName, first);
            if (e.SessionSeq.HasValue) { Sep(sb, ref first); sb.Append("\"sessionSeq\":").Append(e.SessionSeq.Value); }
            if (e.DurationMs.HasValue) { Sep(sb, ref first); sb.Append("\"durationMs\":").Append(e.DurationMs.Value); }
            first = WriteStringIfNotNull(sb, "amount", e.Amount, first);
            if (e.Quantity.HasValue) { Sep(sb, ref first); sb.Append("\"quantity\":").Append(e.Quantity.Value); }
            if (e.Score.HasValue) { Sep(sb, ref first); sb.Append("\"score\":").Append(e.Score.Value); }
            first = WriteDoubleMap(sb, "metrics", e.Metrics, first);

            // oneof payload
            if (e.Behavior != null)
            {
                Sep(sb, ref first);
                sb.Append("\"behavior\":");
                AppendBehavior(sb, e.Behavior);
            }
            if (e.Risk != null)
            {
                Sep(sb, ref first);
                sb.Append("\"risk\":");
                AppendRisk(sb, e.Risk);
            }
            sb.Append('}');
        }

        private static void AppendBehavior(StringBuilder sb, BehaviorEvent b)
        {
            sb.Append('{');
            bool first = true;
            first = WriteStringIfNotNull(sb, "eventAction", b.EventAction, first);
            first = WriteStringIfNotNull(sb, "objectType", b.ObjectType, first);
            first = WriteStringIfNotNull(sb, "objectId", b.ObjectId, first);
            first = WriteStringIfNotNull(sb, "objectName", b.ObjectName, first);
            if (b.SessionSeq.HasValue) { Sep(sb, ref first); sb.Append("\"sessionSeq\":").Append(b.SessionSeq.Value); }
            first = WriteStringIfNotNull(sb, "os", b.Os, first);
            first = WriteStringIfNotNull(sb, "appVersion", b.AppVersion, first);
            first = WriteStringIfNotNull(sb, "channel", b.Channel, first);
            first = WriteStringIfNotNull(sb, "network", b.Network, first);
            if (b.DurationMs.HasValue) { Sep(sb, ref first); sb.Append("\"durationMs\":").Append(b.DurationMs.Value); }
            first = WriteStringIfNotNull(sb, "amount", b.Amount, first);
            if (b.Quantity.HasValue) { Sep(sb, ref first); sb.Append("\"quantity\":").Append(b.Quantity.Value); }
            if (b.Score.HasValue) { Sep(sb, ref first); sb.Append("\"score\":").Append(b.Score.Value); }
            first = WriteDoubleMap(sb, "metrics", b.Metrics, first);
            first = WriteStringIfNotNull(sb, "opResult", b.OpResult, first);
            WriteStringIfNotNull(sb, "errorCode", b.ErrorCode, first);
            sb.Append('}');
        }

        private static void AppendRisk(StringBuilder sb, RiskEvent r)
        {
            sb.Append('{');
            bool first = true;
            first = WriteStringIfNotNull(sb, "riskEventId", r.RiskEventId, first);
            first = WriteStringIfNotNull(sb, "riskType", r.RiskType, first);
            first = WriteStringIfNotNull(sb, "riskLevel", r.RiskLevel, first);
            if (r.RiskScore.HasValue) { Sep(sb, ref first); sb.Append("\"riskScore\":").Append(r.RiskScore.Value.ToString(CultureInfo.InvariantCulture)); }
            if (r.RuleId.HasValue) { Sep(sb, ref first); sb.Append("\"ruleId\":").Append(r.RuleId.Value); }
            first = WriteStringIfNotNull(sb, "ruleName", r.RuleName, first);
            if (r.RelatedEventIds != null && r.RelatedEventIds.Count > 0)
            {
                Sep(sb, ref first);
                sb.Append("\"relatedEventIds\":[");
                for (int i = 0; i < r.RelatedEventIds.Count; i++)
                {
                    if (i > 0) sb.Append(',');
                    AppendQuoted(sb, r.RelatedEventIds[i]);
                }
                sb.Append(']');
            }
            first = WriteStringIfNotNull(sb, "description", r.Description, first);
            first = WriteStringMap(sb, "evidence", r.Evidence, first);
            if (r.Status.HasValue) { Sep(sb, ref first); sb.Append("\"status\":\"").Append(r.Status.Value).Append('\"'); }
            if (r.HandlerId.HasValue) { Sep(sb, ref first); sb.Append("\"handlerId\":").Append(r.HandlerId.Value); }
            first = WriteStringIfNotNull(sb, "handleRemark", r.HandleRemark, first);
            first = WriteStringIfNotNull(sb, "occurTime", r.OccurTime, first);
            WriteStringIfNotNull(sb, "reportTime", r.ReportTime, first);
            sb.Append('}');
        }

        // ──────────── 反序列化（响应体，只取关键字段）────────────

        public static PostReportResponse? DeserializeResponse(string json)
        {
            if (string.IsNullOrEmpty(json)) return null;
            return new PostReportResponse
            {
                Success = JsonRead.Bool(json, "success"),
                Message = JsonRead.String(json, "message"),
                RequestId = JsonRead.String(json, "requestId"),
                ServerTime = JsonRead.Long(json, "serverTime"),
                TotalCount = JsonRead.Int(json, "totalCount"),
                SuccessCount = JsonRead.Int(json, "successCount"),
                FailedCount = JsonRead.Int(json, "failedCount"),
            };
        }

        public static KratosError? DeserializeError(string json)
        {
            if (string.IsNullOrEmpty(json)) return null;
            return new KratosError
            {
                Code = JsonRead.Int(json, "code") ?? 0,
                Reason = JsonRead.String(json, "reason"),
                Message = JsonRead.String(json, "message"),
            };
        }

        // ──────────── 辅助写入 ────────────

        private static void Sep(StringBuilder sb, ref bool first)
        {
            if (first) first = false;
            else sb.Append(',');
        }

        private static void WriteString(StringBuilder sb, string key, string? value)
        {
            sb.Append('"').Append(key).Append("\":");
            AppendQuoted(sb, value);
        }

        private static bool WriteStringIfNotNull(StringBuilder sb, string key, string? value, bool first)
        {
            if (value == null) return first;
            Sep(sb, ref first);
            WriteString(sb, key, value);
            return false;
        }

        private static bool WriteStringMap(StringBuilder sb, string key, Dictionary<string, string>? map, bool first)
        {
            if (map == null || map.Count == 0) return first;
            Sep(sb, ref first);
            sb.Append('"').Append(key).Append("\":{");
            int i = 0;
            foreach (var kv in map)
            {
                if (i++ > 0) sb.Append(',');
                AppendQuoted(sb, kv.Key);
                sb.Append(':');
                AppendQuoted(sb, kv.Value);
            }
            sb.Append('}');
            return false;
        }

        private static bool WriteDoubleMap(StringBuilder sb, string key, Dictionary<string, double>? map, bool first)
        {
            if (map == null || map.Count == 0) return first;
            Sep(sb, ref first);
            sb.Append('"').Append(key).Append("\":{");
            int i = 0;
            foreach (var kv in map)
            {
                if (i++ > 0) sb.Append(',');
                AppendQuoted(sb, kv.Key);
                sb.Append(':');
                sb.Append(kv.Value.ToString("R", CultureInfo.InvariantCulture));
            }
            sb.Append('}');
            return false;
        }

        private static void AppendQuoted(StringBuilder sb, string? s)
        {
            sb.Append('"');
            if (!string.IsNullOrEmpty(s))
            {
                foreach (char c in s)
                {
                    switch (c)
                    {
                        case '"': sb.Append("\\\""); break;
                        case '\\': sb.Append("\\\\"); break;
                        case '\b': sb.Append("\\b"); break;
                        case '\f': sb.Append("\\f"); break;
                        case '\n': sb.Append("\\n"); break;
                        case '\r': sb.Append("\\r"); break;
                        case '\t': sb.Append("\\t"); break;
                        default:
                            if (c < 0x20)
                            {
                                sb.Append('\\').Append('u').Append(((int)c).ToString("x4", CultureInfo.InvariantCulture));
                            }
                            else
                            {
                                sb.Append(c);
                            }
                            break;
                    }
                }
            }
            sb.Append('"');
        }
    }

    /// <summary>极简 JSON 值读取（按 key 查找标量，不解析完整结构）</summary>
    internal static class JsonRead
    {
        public static bool Bool(string json, string key)
        {
            var v = FindValue(json, key);
            return v == "true";
        }

        public static string? String(string json, string key)
        {
            var v = FindValue(json, key);
            if (v == null || v.Length < 2) return null;
            if (v[0] != '"' || v[v.Length - 1] != '"') return null;
            return Unescape(v.Substring(1, v.Length - 2));
        }

        public static int? Int(string json, string key)
        {
            var v = FindValue(json, key);
            if (v == null || v == "null") return null;
            if (int.TryParse(v, NumberStyles.Integer, CultureInfo.InvariantCulture, out var n)) return n;
            return null;
        }

        public static long? Long(string json, string key)
        {
            var v = FindValue(json, key);
            if (v == null || v == "null") return null;
            if (long.TryParse(v, NumberStyles.Integer, CultureInfo.InvariantCulture, out var n)) return n;
            return null;
        }

        // 查找 "key":<value> 中的 value 原始片段
        private static string? FindValue(string json, string key)
        {
            string pattern = "\"" + key + "\"";
            int idx = json.IndexOf(pattern, System.StringComparison.Ordinal);
            if (idx < 0) return null;
            int colon = json.IndexOf(':', idx + pattern.Length);
            if (colon < 0) return null;
            int i = colon + 1;
            while (i < json.Length && (json[i] == ' ' || json[i] == '\t' || json[i] == '\n' || json[i] == '\r')) i++;
            if (i >= json.Length) return null;

            if (json[i] == '"')
            {
                int end = i + 1;
                while (end < json.Length)
                {
                    if (json[end] == '\\') { end += 2; continue; }
                    if (json[end] == '"') break;
                    end++;
                }
                return json.Substring(i, end - i + 1);
            }
            // 数字 / true / false / null
            int e = i;
            while (e < json.Length && json[e] != ',' && json[e] != '}' && json[e] != ']' &&
                   json[e] != ' ' && json[e] != '\n' && json[e] != '\r')
            {
                e++;
            }
            return json.Substring(i, e - i);
        }

        private static string Unescape(string s)
        {
            if (s.IndexOf('\\') < 0) return s;
            var sb = new StringBuilder(s.Length);
            for (int i = 0; i < s.Length; i++)
            {
                if (s[i] == '\\' && i + 1 < s.Length)
                {
                    char c = s[++i];
                    switch (c)
                    {
                        case '"': sb.Append('"'); break;
                        case '\\': sb.Append('\\'); break;
                        case '/': sb.Append('/'); break;
                        case 'b': sb.Append('\b'); break;
                        case 'f': sb.Append('\f'); break;
                        case 'n': sb.Append('\n'); break;
                        case 'r': sb.Append('\r'); break;
                        case 't': sb.Append('\t'); break;
                        case 'u':
                            if (i + 4 < s.Length)
                            {
                                int code = int.Parse(s.Substring(i + 1, 4), NumberStyles.HexNumber, CultureInfo.InvariantCulture);
                                sb.Append((char)code);
                                i += 4;
                            }
                            break;
                        default: sb.Append(c); break;
                    }
                }
                else
                {
                    sb.Append(s[i]);
                }
            }
            return sb.ToString();
        }
    }
}
