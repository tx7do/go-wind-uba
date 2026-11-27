// uba_core.cpp —— C++ 核心层实现（对齐 Go/Java/Python 服务端 SDK 语义）。
#include "uba_core.h"

#include <atomic>
#include <chrono>
#include <condition_variable>
#include <cstdlib>
#include <set>
#include <cstdio>
#include <functional>
#include <sstream>
#include <utility>

namespace uba {

namespace {

int64_t SystemNowMs() {
    return std::chrono::duration_cast<std::chrono::milliseconds>(
               std::chrono::system_clock::now().time_since_epoch())
        .count();
}

std::string TrimSlash(std::string s) {
    while (!s.empty() && s.back() == '/') s.pop_back();
    return s;
}

std::string JsonEscape(const std::string& s) {
    std::string out;
    out.reserve(s.size() + 2);
    out.push_back('"');
    for (unsigned char c : s) {
        switch (c) {
            case '"': out += "\\\""; break;
            case '\\': out += "\\\\"; break;
            case '\n': out += "\\n"; break;
            case '\r': out += "\\r"; break;
            case '\t': out += "\\t"; break;
            case '\b': out += "\\b"; break;
            case '\f': out += "\\f"; break;
            default:
                if (c < 0x20) {
                    char buf[8];
                    std::snprintf(buf, sizeof(buf), "\\u%04x", c);
                    out += buf;
                } else {
                    out.push_back(static_cast<char>(c));
                }
        }
    }
    out.push_back('"');
    return out;
}

std::string NumToString(double d) {
    // 整数值不带小数点（对齐其他端 SDK 行为）
    if (d == static_cast<double>(static_cast<int64_t>(d)) && std::abs(d) < 1e15) {
        return std::to_string(static_cast<int64_t>(d));
    }
    std::ostringstream oss;
    oss << d;
    return oss.str();
}

// wire JSON 便捷构造：按插入顺序产出对象字符串。
class WireObject {
public:
    void Raw(const std::string& key, const std::string& wire_value) {
        if (wire_value.empty()) return;
        Sep();
        parts_ += JsonEscape(key) + ":" + wire_value;
    }
    void Str(const std::string& key, const std::optional<std::string>& v) {
        if (!v.has_value() || v->empty()) return;
        Raw(key, JsonEscape(*v));
    }
    void Int(const std::string& key, const std::optional<int64_t>& v) {
        if (!v.has_value()) return;
        Raw(key, std::to_string(*v));
    }
    void Int32(const std::string& key, const std::optional<int32_t>& v) {
        if (!v.has_value()) return;
        Raw(key, std::to_string(*v));
    }
    void Double(const std::string& key, const std::optional<double>& v) {
        if (!v.has_value()) return;
        Raw(key, NumToString(*v));
    }
    void DoubleMap(const std::string& key, const std::map<std::string, double>& m) {
        if (m.empty()) return;
        std::string inner;
        bool first = true;
        for (const auto& [k, v] : m) {
            if (!first) inner += ",";
            first = false;
            inner += JsonEscape(k) + ":" + NumToString(v);
        }
        Raw(key, "{" + inner + "}");
    }
    void StrMap(const std::string& key, const std::map<std::string, std::string>& m) {
        if (m.empty()) return;
        std::string inner;
        bool first = true;
        for (const auto& [k, v] : m) {
            if (!first) inner += ",";
            first = false;
            inner += JsonEscape(k) + ":" + JsonEscape(v);
        }
        Raw(key, "{" + inner + "}");
    }
    void JsonValue(const std::string& key, const UbaJsonValue& v) {
        Raw(key, JsonValueString(v));
    }
    void StrArray(const std::string& key, const std::vector<std::string>& v) {
        if (v.empty()) return;
        std::string inner;
        for (std::size_t i = 0; i < v.size(); ++i) {
            if (i > 0) inner += ",";
            inner += JsonEscape(v[i]);
        }
        Raw(key, "[" + inner + "]");
    }

    std::string Build() {
        std::string out = "{";
        // parts_ 已经带逗号分隔，直接拼接
        out += parts_;
        out += "}";
        return out;
    }

private:
    void Sep() {
        if (!parts_.empty()) parts_ += ",";
    }
    static std::string JsonValueString(const UbaJsonValue& v) {
        struct Visitor {
            std::string operator()(std::monostate) const { return "null"; }
            std::string operator()(bool b) const { return b ? "true" : "false"; }
            std::string operator()(int64_t i) const { return std::to_string(i); }
            std::string operator()(double d) const { return NumToString(d); }
            std::string operator()(const std::string& s) const { return JsonEscape(s); }
            std::string operator()(const UbaJsonArray& arr) const {
                std::string inner;
                for (std::size_t i = 0; i < arr.size(); ++i) {
                    if (i > 0) inner += ",";
                    inner += JsonValueString(arr[i]);
                }
                return "[" + inner + "]";
            }
            std::string operator()(const UbaJsonObject& obj) const {
                std::string inner;
                bool first = true;
                for (const auto& [k, val] : obj) {
                    if (!first) inner += ",";
                    first = false;
                    inner += JsonEscape(k) + ":" + JsonValueString(val);
                }
                return "{" + inner + "}";
            }
        };
        return std::visit(Visitor{}, v);
    }

    std::string parts_;
};

std::string BehaviorToWire(const UbaTrackOptions& o, const std::string& os_name,
                           const std::string& app_version, const std::string& channel) {
    WireObject b;
    b.Raw("os", JsonEscape(os_name));
    if (!app_version.empty()) b.Raw("appVersion", JsonEscape(app_version));
    if (!channel.empty()) b.Raw("channel", JsonEscape(channel));
    b.Str("eventAction", o.event_action);
    b.Str("objectType", o.object_type);
    b.Str("objectId", o.object_id);
    b.Str("objectName", o.object_name);
    b.Str("network", o.network);
    b.Int("durationMs", o.duration_ms);
    b.Str("amount", o.amount);
    b.Int("quantity", o.quantity);
    b.Int32("score", o.score);
    b.DoubleMap("metrics", o.metrics);
    b.Str("opResult", o.op_result);
    b.Str("errorCode", o.error_code);
    b.Str("serverId", o.server_id);
    b.Int32("level", o.level);
    return b.Build();
}

std::string RiskToWire(const UbaRiskEvent& r) {
    WireObject m;
    m.Str("riskType", r.risk_type);
    m.Str("riskLevel", r.risk_level);
    m.Double("riskScore", r.risk_score);
    m.Int("ruleId", r.rule_id);
    m.Str("ruleName", r.rule_name);
    if (r.rule_context.has_value()) m.JsonValue("ruleContext", *r.rule_context);
    if (r.related_event_ids.has_value()) m.StrArray("relatedEventIds", *r.related_event_ids);
    m.Str("description", r.description);
    m.StrMap("evidence", r.evidence);
    m.Str("status", r.status);
    return m.Build();
}

std::string MergeStrMaps(const std::map<std::string, std::string>& base,
                         const std::map<std::string, std::string>& extra) {
    std::map<std::string, std::string> merged = base;
    for (const auto& [k, v] : extra) merged[k] = v;
    std::string inner;
    bool first = true;
    for (const auto& [k, v] : merged) {
        if (!first) inner += ",";
        first = false;
        inner += JsonEscape(k) + ":" + JsonEscape(v);
    }
    return "{" + inner + "}";
}

// 轻量提取 "failedCount":N
int ParseFailedCount(const std::string& body) {
    const std::string key = "\"failedCount\"";
    std::size_t idx = body.find(key);
    if (idx == std::string::npos) return 0;
    idx = body.find(':', idx + key.size());
    if (idx == std::string::npos) return 0;
    std::string digits;
    for (std::size_t i = idx + 1; i < body.size(); ++i) {
        char c = body[i];
        if (c >= '0' && c <= '9') digits += c;
        else if (!digits.empty()) break;
        else if (c != ' ' && c != '\t') break;
    }
    if (digits.empty()) return 0;
    return std::atoi(digits.c_str());
}

// 轻量提取 {"flags":{...}} 键值对
std::map<std::string, std::string> ParseFlags(const std::string& body) {
    std::map<std::string, std::string> out;
    const std::string key = "\"flags\"";
    std::size_t flags_idx = body.find(key);
    if (flags_idx == std::string::npos) return out;
    std::size_t obj_start = body.find('{', flags_idx);
    if (obj_start == std::string::npos) return out;
    int depth = 0;
    std::size_t obj_end = std::string::npos;
    for (std::size_t i = obj_start; i < body.size(); ++i) {
        if (body[i] == '{') depth++;
        else if (body[i] == '}') {
            depth--;
            if (depth == 0) {
                obj_end = i;
                break;
            }
        }
    }
    if (obj_end == std::string::npos) return out;
    std::size_t inner_start = obj_start + 1;
    std::size_t i = inner_start;
    while (i < obj_end) {
        std::size_t key_start = body.find('"', i);
        if (key_start == std::string::npos || key_start >= obj_end) break;
        std::size_t key_end = body.find('"', key_start + 1);
        if (key_end == std::string::npos) break;
        std::size_t colon = body.find(':', key_end + 1);
        if (colon == std::string::npos) break;
        std::size_t val_start = body.find('"', colon + 1);
        if (val_start == std::string::npos) break;
        std::size_t val_end = body.find('"', val_start + 1);
        if (val_end == std::string::npos) break;
        out[body.substr(key_start + 1, key_end - key_start - 1)] =
            body.substr(val_start + 1, val_end - val_start - 1);
        i = val_end + 1;
        while (i < obj_end && (body[i] == ',' || body[i] == ' ')) i++;
    }
    return out;
}

std::string DefaultHostname() {
    // C++ 标准库无主机名 API：由平台层（Unreal 插件）填 config.device_id；
    // 核心层兜底固定占位。
    return "cpp-host";
}

} // namespace

// ─── Config ─────────────────────────────────────────────────────────────────

void UbaConfig::Validate() const {
    if (app_id.empty()) throw std::invalid_argument("app_id 不能为空");
    if (app_secret.empty()) throw std::invalid_argument("app_secret 不能为空");
    if (endpoint.empty()) throw std::invalid_argument("endpoint 不能为空");
    if (timeout_ms <= 0 || timeout_ms >= 10000)
        throw std::invalid_argument("timeout_ms 必须 > 0 且 < 10000（服务端超时 10s）");
    if (max_batch_bytes <= 0 || max_batch_bytes >= 1024 * 1024)
        throw std::invalid_argument("max_batch_bytes 必须压在 1MiB 以内");
    if (batch_size <= 0) throw std::invalid_argument("batch_size 必须 > 0");
    if (sample_rate < 0.0 || sample_rate > 1.0)
        throw std::invalid_argument("sample_rate 必须在 0~1");
}

std::string UbaConfig::ReportUrl() const { return TrimSlash(endpoint) + path; }
std::string UbaConfig::FlagsUrl() const { return TrimSlash(endpoint) + flags_path; }

int64_t UbaRealClock::NowMs() { return SystemNowMs(); }

std::string UbaClient::NewUuid() {
    // 无 RNG 库依赖：chrono 熵 + FNV 混淆（uuid 仅作去重键，不要求密码学强度）
    static std::atomic<uint64_t> counter{0};
    uint64_t seed = SystemNowMs() ^ (static_cast<uint64_t>(counter.fetch_add(1)) << 24) ^
                    (static_cast<uint64_t>(std::hash<std::thread::id>{}(
                         std::this_thread::get_id())) << 32);
    uint8_t bytes[16];
    for (int i = 0; i < 2; ++i) {
        uint64_t h = 0xcbf29ce484222325ULL ^ (seed + static_cast<uint64_t>(i) * 0x9E3779B97F4A7C15ULL);
        for (int round = 0; round < 8; ++round) {
            h ^= (seed >> (round * 8)) & 0xff;
            h *= 0x100000001b3ULL;
        }
        for (int b = 0; b < 8; ++b) bytes[i * 8 + b] = static_cast<uint8_t>((h >> (b * 8)) & 0xff);
    }
    bytes[6] = static_cast<uint8_t>((bytes[6] & 0x0f) | 0x40);
    bytes[8] = static_cast<uint8_t>((bytes[8] & 0x3f) | 0x80);
    char buf[40];
    std::snprintf(buf, sizeof(buf),
                  "%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",
                  bytes[0], bytes[1], bytes[2], bytes[3], bytes[4], bytes[5], bytes[6],
                  bytes[7], bytes[8], bytes[9], bytes[10], bytes[11], bytes[12], bytes[13],
                  bytes[14], bytes[15]);
    return buf;
}

std::string UbaClient::ToRFC3339Ms(int64_t epoch_ms) {
    std::time_t seconds = static_cast<std::time_t>(epoch_ms / 1000);
    int ms = static_cast<int>(epoch_ms % 1000);
    std::tm tm_utc{};
#ifdef _WIN32
    gmtime_s(&tm_utc, &seconds);
#else
    gmtime_r(&seconds, &tm_utc);
#endif
    char buf[40];
    std::snprintf(buf, sizeof(buf), "%04d-%02d-%02dT%02d:%02d:%02d.%03dZ",
                  tm_utc.tm_year + 1900, tm_utc.tm_mon + 1, tm_utc.tm_mday,
                  tm_utc.tm_hour, tm_utc.tm_min, tm_utc.tm_sec, ms);
    return buf;
}

uint32_t UbaClient::Fnv1a32(const std::string& s) {
    uint32_t h = 0x811c9dc5u;
    for (unsigned char c : s) {
        h ^= c;
        h *= 16777619u;
    }
    return h;
}

bool UbaClient::SessionInSample(const std::string& session_id, double rate) {
    if (rate >= 1.0) return true;
    if (rate <= 0.0) return false;
    return static_cast<double>(Fnv1a32(session_id) % 1000) / 1000.0 < rate;
}

// ─── Client Impl ────────────────────────────────────────────────────────────

struct UbaClient::Impl {
    UbaConfig config;
    std::shared_ptr<IUbaTransport> transport;
    std::shared_ptr<IUbaClock> clock;

    std::mutex mutex;
    std::deque<std::string> wire_queue;   // 已序列化的 wire 事件（失败回队/补发无损）
    bool flushing = false;
    bool closed = false;

    std::optional<int64_t> user_id;
    std::map<std::string, std::string> super_props;
    std::map<std::string, std::string> flags;
    std::set<std::string> exposed_flags;
    std::string exposed_session;
    std::string session_id;
    std::optional<int64_t> last_event_at;
    std::optional<int64_t> last_screen_at;
    std::optional<int64_t> session_paused_at;

    std::thread timer;
    bool timer_running = false;
    std::condition_variable timer_cv;
    std::string user_agent;
    std::string device_id;
    std::string os_name;

    Impl(UbaConfig cfg, std::shared_ptr<IUbaTransport> t, std::shared_ptr<IUbaClock> c)
        : config(std::move(cfg)), transport(std::move(t)), clock(std::move(c)) {
        if (!clock) clock = std::make_shared<UbaRealClock>();
        config.Validate();
        device_id = config.device_id.empty() ? DefaultHostname() : config.device_id;
        os_name = "Cpp";
        user_agent = std::string("uba-cpp/") + kSdkVersion + " (" + os_name + ")";
        session_id = NewUuid();
    }

    void StartTimer(std::function<void()> flush_fn) {
        if (config.flush_interval_ms <= 0) return;
        timer_running = true;
        timer = std::thread([this, flush_fn] {
            std::unique_lock<std::mutex> lk(mutex);
            auto deadline = std::chrono::steady_clock::now() +
                            std::chrono::milliseconds(config.flush_interval_ms);
            while (true) {
                if (timer_cv.wait_until(lk, deadline, [this] { return !timer_running; })) {
                    return;
                }
                bool has_pending = !wire_queue.empty();
                bool is_closed = closed;
                lk.unlock();
                if (has_pending && !is_closed) {
                    try {
                        flush_fn();
                    } catch (...) {
                    }
                }
                if (is_closed) return;
                deadline = std::chrono::steady_clock::now() +
                           std::chrono::milliseconds(config.flush_interval_ms);
                lk.lock();
            }
        });
    }

    void StopTimer() {
        {
            std::lock_guard<std::mutex> lk(mutex);
            if (!timer_running) return;
            timer_running = false;
        }
        timer_cv.notify_all();
        if (timer.joinable()) timer.join();
    }

    std::string CurrentSessionLocked() {
        int64_t now = clock->NowMs();
        if (config.session_timeout_ms > 0 && last_event_at.has_value() &&
            now - *last_event_at > config.session_timeout_ms) {
            session_id = NewUuid();
            exposed_session.clear();
        }
        last_event_at = now;
        return session_id;
    }

    bool ShouldCollect(const std::string& event_name, const UbaTrackOptions* o,
                       std::string* session_out) {
        if (closed) {
            Debug("客户端已关闭，丢弃事件 " + event_name);
            return false;
        }
        std::string session;
        if (o != nullptr && o->session_id.has_value()) {
            session = *o->session_id;
        } else {
            session = CurrentSessionLocked();
        }
        if (session_out != nullptr) *session_out = session;
        return SessionInSample(session, config.sample_rate);
    }

    std::map<std::string, std::string> SuperPropsLocked() { return super_props; }

    void Debug(const std::string& msg) {
        if (config.debug) std::fprintf(stderr, "[uba-cpp] %s\n", msg.c_str());
    }
};

UbaClient::UbaClient(UbaConfig config, std::shared_ptr<IUbaTransport> transport,
                     std::shared_ptr<IUbaClock> clock)
    : impl_(new Impl(std::move(config), std::move(transport), std::move(clock))) {
    impl_->StartTimer([this] {
        try {
            Flush();
        } catch (...) {
        }
    });
    // 首装 app_launch 由平台层（Unreal 插件）判定后调用 Track("app_launch", ...)
}

UbaClient::~UbaClient() { Close(); }

void UbaClient::Identify(int64_t user_id) {
    if (user_id == 0) {
        ResetUser();
        return;
    }
    std::lock_guard<std::mutex> lk(impl_->mutex);
    impl_->user_id = user_id;
}

void UbaClient::ResetUser() {
    std::lock_guard<std::mutex> lk(impl_->mutex);
    impl_->user_id.reset();
}

void UbaClient::SetSuperProperties(std::map<std::string, std::string> props) {
    std::lock_guard<std::mutex> lk(impl_->mutex);
    for (const auto& [k, v] : props) impl_->super_props[k] = v;
}

void UbaClient::ClearSuperProperties() {
    std::lock_guard<std::mutex> lk(impl_->mutex);
    impl_->super_props.clear();
}

void UbaClient::Track(const std::string& event_name,
                      const std::map<std::string, std::string>& properties,
                      const UbaTrackOptions& options) {
    std::string session;
    if (!impl_->ShouldCollect(event_name, &options, &session)) return;

    const int64_t uid = options.user_id.value_or(
        impl_->user_id.value_or(0));
    const std::string device = options.device_id.value_or(impl_->device_id);
    const std::string platform = options.platform.value_or(impl_->config.platform);

    WireObject top;
    top.Raw("eventType", JsonEscape("BEHAVIOR"));
    top.Raw("eventId", JsonEscape(NewUuid()));
    top.Raw("eventName", JsonEscape(event_name));
    top.Raw("eventTime", JsonEscape(ToRFC3339Ms(impl_->clock->NowMs())));
    if (uid != 0) top.Int("userId", uid);
    top.Raw("deviceId", JsonEscape(device));
    top.Raw("sessionId", JsonEscape(session));
    top.Raw("platform", JsonEscape(platform));
    top.Str("eventCategory", options.event_category);
    if (!properties.empty() || !options.properties.empty() || !impl_->super_props.empty()) {
        std::map<std::string, std::string> call_props = properties;
                for (const auto& [k, v] : options.properties) call_props[k] = v;
                std::map<std::string, std::string> base_props = impl_->SuperPropsLocked();
                std::string props_wire = MergeStrMaps(base_props, call_props);
                if (props_wire != "{}") top.Raw("properties", props_wire);
    }
    if (!options.metrics.empty()) top.DoubleMap("metrics", options.metrics);
    top.Str("traceId", options.trace_id);
    // 顶层字符串镜像维度；数值标量只进 behavior oneof
    top.Str("eventAction", options.event_action);
    top.Str("objectType", options.object_type);
    top.Str("objectId", options.object_id);
    top.Str("objectName", options.object_name);
    top.Str("amount", options.amount);
    top.Str("opResult", options.op_result);
    top.Str("errorCode", options.error_code);
    top.Raw("behavior",
            BehaviorToWire(options, impl_->os_name, impl_->config.app_version,
                           impl_->config.channel));

    Enqueue(top.Build());
}

void UbaClient::TrackRisk(const std::string& event_name, const UbaRiskEvent& risk,
                          const std::map<std::string, std::string>& properties,
                          const UbaTrackOptions& options) {
    std::string session;
    if (!impl_->ShouldCollect(event_name, &options, &session)) return;
    const int64_t uid = options.user_id.value_or(impl_->user_id.value_or(0));
    const std::string device = options.device_id.value_or(impl_->device_id);
    const std::string platform = options.platform.value_or(impl_->config.platform);

    WireObject top;
    top.Raw("eventType", JsonEscape("RISK"));
    top.Raw("eventId", JsonEscape(NewUuid()));
    top.Raw("eventName", JsonEscape(event_name));
    top.Raw("eventTime", JsonEscape(ToRFC3339Ms(impl_->clock->NowMs())));
    if (uid != 0) top.Int("userId", uid);
    top.Raw("deviceId", JsonEscape(device));
    top.Raw("sessionId", JsonEscape(session));
    top.Raw("platform", JsonEscape(platform));
    top.Str("eventCategory", options.event_category);
    if (!properties.empty() || !options.properties.empty() || !impl_->super_props.empty()) {
        std::map<std::string, std::string> call_props = properties;
                for (const auto& [k, v] : options.properties) call_props[k] = v;
                std::map<std::string, std::string> base_props = impl_->SuperPropsLocked();
                std::string props_wire = MergeStrMaps(base_props, call_props);
                if (props_wire != "{}") top.Raw("properties", props_wire);
    }
    if (!options.metrics.empty()) top.DoubleMap("metrics", options.metrics);
    top.Str("traceId", options.trace_id);
    top.Raw("risk", RiskToWire(risk));

    Enqueue(top.Build());
}

void UbaClient::TrackPageView(const std::string& screen_name, const std::string& title,
                              const std::string& referer) {
    const int64_t now = impl_->clock->NowMs();
    std::optional<int64_t> dwell;
    if (impl_->last_screen_at.has_value()) dwell = now - *impl_->last_screen_at;
    impl_->last_screen_at = now;
    UbaTrackOptions o;
    o.event_category = "navigation";
    o.object_type = "page";
    o.object_id = screen_name;
    if (dwell.has_value() && *dwell > 0) o.duration_ms = *dwell;
    std::map<std::string, std::string> props;
    if (!title.empty()) props["title"] = title;
    if (!referer.empty()) props["referer"] = referer;
    props["pageUrl"] = screen_name;
    Track(impl_->config.page_view_event.empty() ? "page_view" : impl_->config.page_view_event,
          props, o);
}

bool UbaClient::IsCollecting() const {
    std::lock_guard<std::mutex> lk(impl_->mutex);
    return !impl_->closed && SessionInSample(impl_->session_id, impl_->config.sample_rate);
}

std::size_t UbaClient::PendingCount() const {
    std::lock_guard<std::mutex> lk(impl_->mutex);
    return impl_->wire_queue.size();
}

std::string UbaClient::SessionId() const {
    std::lock_guard<std::mutex> lk(impl_->mutex);
    return impl_->session_id;
}

void UbaClient::Enqueue(const std::string& wire) {
    bool auto_flush = false;
    {
        std::lock_guard<std::mutex> lk(impl_->mutex);
        impl_->wire_queue.push_back(wire);
        if (impl_->wire_queue.size() > impl_->config.max_queue_size) {
            std::size_t excess = impl_->wire_queue.size() - impl_->config.max_queue_size;
            impl_->wire_queue.erase(impl_->wire_queue.begin(),
                                    impl_->wire_queue.begin() + static_cast<long>(excess));
        }
        auto_flush = impl_->wire_queue.size() >= static_cast<std::size_t>(impl_->config.batch_size);
    }
    if (auto_flush) {
        std::thread([this] {
            try {
                Flush();
            } catch (...) {
            }
        }).detach();
    }
}

UbaFlushResult UbaClient::Flush() {
    {
        std::lock_guard<std::mutex> lk(impl_->mutex);
        if (impl_->flushing || impl_->closed) return UbaFlushResult{};
        impl_->flushing = true;
    }
    int sent = 0;
    int dropped = 0;
    bool ok = true;
    bool retryable = false;
    std::string message;
    try {
        while (true) {
            std::vector<std::string> batch;
            int bytes = 0;
            {
                std::lock_guard<std::mutex> lk(impl_->mutex);
                while (!impl_->wire_queue.empty() &&
                       static_cast<int>(batch.size()) < impl_->config.batch_size) {
                    const std::string& next = impl_->wire_queue.front();
                    int encoded = static_cast<int>(next.size());
                    if (!batch.empty() && bytes + encoded > impl_->config.max_batch_bytes) break;
                    bytes += encoded;
                    batch.push_back(next);
                    impl_->wire_queue.pop_front();
                }
            }
            if (batch.empty()) break;

            UbaFlushResult r = SendWithRetry(batch);
            sent += r.sent;
            dropped += r.dropped;
            if (!r.success) {
                ok = false;
                retryable = r.retryable;
                message = r.message;
                if (retryable) {
                    std::lock_guard<std::mutex> lk(impl_->mutex);
                    for (auto it = batch.rbegin(); it != batch.rend(); ++it) {
                        impl_->wire_queue.push_front(*it);
                    }
                }
                break;
            }
        }
    } catch (...) {
        std::lock_guard<std::mutex> lk(impl_->mutex);
        impl_->flushing = false;
        throw;
    }
    {
        std::lock_guard<std::mutex> lk(impl_->mutex);
        impl_->flushing = false;
    }
    return UbaFlushResult{ok, sent, dropped, retryable, message};
}

UbaFlushResult UbaClient::SendWithRetry(const std::vector<std::string>& batch) {
    std::string body = "{\"appId\":" + JsonEscape(impl_->config.app_id) +
                       ",\"appSecret\":" + JsonEscape(impl_->config.app_secret) +
                       ",\"clientInfo\":{\"userAgent\":" + JsonEscape(impl_->user_agent) + "}";
    // events 数组拼接
    std::string events = "[";
    for (std::size_t i = 0; i < batch.size(); ++i) {
        if (i > 0) events += ",";
        events += batch[i];
    }
    events += "]";
    // 把 events 插到 envelope 末尾（对象字段顺序无关，直接拼 {.., "events":[..]}
    body.pop_back(); // 去掉 '}'
    body += ",\"events\":" + events + "}";

    for (int attempt = 0;; ++attempt) {
        UbaHttpResult res = impl_->transport->Send(impl_->config.ReportUrl(), body,
                                                   impl_->config.timeout_ms);
        if (res.IsOk()) {
            int failed = ParseFailedCount(res.body);
            if (failed > 0) impl_->Debug("部分事件被服务端拒绝：failedCount=" + std::to_string(failed));
            return UbaFlushResult{true, static_cast<int>(batch.size()) - failed, failed, false, ""};
        }
        // 4xx（除 429）是确定性失败：整批丢弃，不重试
        if (res.status_code >= 400 && res.status_code < 500 && res.status_code != 429) {
            impl_->Debug("HTTP " + std::to_string(res.status_code) + "（不可重试），丢弃 " +
                         std::to_string(batch.size()) + " 条");
            return UbaFlushResult{false, 0, static_cast<int>(batch.size()), false,
                                  "HTTP " + std::to_string(res.status_code)};
        }
        if (attempt >= (impl_->config.max_retries < 0 ? 0 : impl_->config.max_retries)) {
            std::string reason = res.status_code == 0 ? res.body
                                                      : "HTTP " + std::to_string(res.status_code);
            impl_->Debug("重试耗尽（" + reason + "），" + std::to_string(batch.size()) +
                         " 条待下次补发");
            return UbaFlushResult{false, 0, static_cast<int>(batch.size()), true,
                                  reason + " after " + std::to_string(attempt + 1) + " attempts"};
        }
        int shift = attempt < 30 ? attempt : 30;
        long long delay = static_cast<long long>(impl_->config.retry_base_delay_ms) << shift;
        impl_->Debug("HTTP " + std::to_string(res.status_code) + "，" +
                     std::to_string(delay) + "ms 后第 " + std::to_string(attempt + 1) + " 次重试");
        std::this_thread::sleep_for(std::chrono::milliseconds(delay));
    }
}

UbaFlushResult UbaClient::Close() {
    impl_->StopTimer();
    UbaFlushResult result = Flush();
    {
        std::lock_guard<std::mutex> lk(impl_->mutex);
        impl_->closed = true;
    }
    impl_->transport->Close();
    return result;
}

UbaFlushResult UbaClient::HandleBackground() {
    impl_->session_paused_at = impl_->clock->NowMs();
    return Flush();
}

void UbaClient::HandleForeground() {
    std::optional<int64_t> paused = impl_->session_paused_at;
    impl_->session_paused_at.reset();
    if (paused.has_value() && impl_->config.session_timeout_ms > 0 &&
        impl_->clock->NowMs() - *paused > impl_->config.session_timeout_ms) {
        std::lock_guard<std::mutex> lk(impl_->mutex);
        impl_->session_id = NewUuid();
        impl_->exposed_session.clear();
    }
}

std::map<std::string, std::string> UbaClient::LoadFlags() {
    std::string body = "{\"appId\":" + JsonEscape(impl_->config.app_id) +
                       ",\"appSecret\":" + JsonEscape(impl_->config.app_secret) +
                       ",\"deviceId\":" + JsonEscape(impl_->device_id);
    std::optional<int64_t> uid;
    {
        std::lock_guard<std::mutex> lk(impl_->mutex);
        uid = impl_->user_id;
    }
    if (uid.has_value() && *uid != 0) body += ",\"userId\":" + std::to_string(*uid);
    body += "}";

    UbaHttpResult res = impl_->transport->Send(impl_->config.FlagsUrl(), body,
                                               impl_->config.timeout_ms);
    std::map<std::string, std::string> out;
    {
        std::lock_guard<std::mutex> lk(impl_->mutex);
        if (res.IsOk()) {
            impl_->flags = ParseFlags(res.body);
        } else {
            impl_->Debug("拉取 flags 失败：HTTP " + std::to_string(res.status_code));
        }
        out = impl_->flags;
    }
    return out;
}

std::optional<std::string> UbaClient::GetFlag(const std::string& key) {
    std::optional<std::string> variant;
    bool first = false;
    {
        std::lock_guard<std::mutex> lk(impl_->mutex);
        auto it = impl_->flags.find(key);
        if (it != impl_->flags.end()) variant = it->second;
        if (variant.has_value()) {
            std::string session = impl_->session_id;
            if (impl_->exposed_session != session) {
                impl_->exposed_flags.clear();
                impl_->exposed_session = session;
            }
            auto inserted = impl_->exposed_flags.insert(key);
            first = inserted.second;
        }
    }
    if (first) {
        std::map<std::string, std::string> props{{"flag_key", key}, {"variant", *variant}};
        UbaTrackOptions o;
        o.event_category = "experiment";
        Track("$feature_flag", props, o);
    }
    return variant;
}

bool UbaClient::IsFeatureEnabled(const std::string& key) {
    std::lock_guard<std::mutex> lk(impl_->mutex);
    auto it = impl_->flags.find(key);
    if (it == impl_->flags.end()) return false;
    return !it->second.empty() && it->second != "control";
}

} // namespace uba
