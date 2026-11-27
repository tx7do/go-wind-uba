// GoWind UBA C++ 核心层（纯 C++17，无 UE 依赖）—— Unreal 插件与其余 C++ 宿主共用。
//
// 契约铁律（与 Web/C#/Flutter/Go/Java/Android/iOS/微信/Python SDK 一致）：
// - 字段 camelCase 对齐后端 proto json_name（键名由 WireJSON 构造时写死）；
// - tenantId / serverTime / ip 服务端补全，SDK 不发送；
// - 数值标量（durationMs/quantity/score/sessionSeq/serverId/level）只进 behavior oneof
//   （proto3 顶层无法区分 unset 0，服务端只信任 oneof 内的值）；
// - 鉴权在请求 body（appId + appSecret），无 Authorization header；
// - 4xx（除 429）确定性失败不重试；网络/5xx/429 耗尽重试后回队补发。
//
// 线程模型：内部 std::mutex 保护队列；定时 flush 由后台线程执行（Close 停止）。
#pragma once

#include <cstdint>
#include <deque>
#include <map>
#include <memory>
#include <mutex>
#include <optional>
#include <string>
#include <thread>
#include <variant>
#include <vector>

namespace uba {

/// 任意 JSON 值（ruleContext 用，对应 google.protobuf.Struct）。
struct UbaJsonValue;
using UbaJsonObject = std::map<std::string, UbaJsonValue>;
using UbaJsonArray = std::vector<UbaJsonValue>;
struct UbaJsonValue
    : std::variant<std::monostate, bool, int64_t, double, std::string, UbaJsonArray,
                   UbaJsonObject> {
    using variant::variant;
};


inline constexpr const char* kSdkVersion = "1.0.0";
inline constexpr int kMaxBatchBytes = 900 * 1024; // < 1MiB：服务端对未鉴权大包直接 401
inline constexpr std::size_t kMaxQueueSize = 10000; // 防持续故障下无界增长

// ─── 配置 ───────────────────────────────────────────────────────────────────

/// SDK 配置。默认值与其他端 SDK 对齐：批量 20 条 / 5s 定时 / 3 次重试 / 1s 退避 / 8s 超时。
struct UbaConfig {
    std::string app_id;                 ///< 应用 ID（管理后台生成），必填
    std::string app_secret;             ///< 应用密钥，鉴权在请求 body 中完成，必填
    std::string endpoint;               ///< collector 地址，如 http://localhost:5700，必填
    std::string path = "/uba/v1/report";
    std::string flags_path = "/uba/v1/flags";
    int batch_size = 20;
    int flush_interval_ms = 5000;       ///< 0 或负数禁用定时器
    int max_retries = 3;                ///< 总尝试 = max_retries + 1
    int timeout_ms = 8000;              ///< 必须 < 服务端 10s
    int retry_base_delay_ms = 1000;
    bool debug = false;
    std::string device_id;              ///< 服务端/宿主实例标识，空则取主机名（Unreal 层注入）
    std::string platform = "server";
    std::string app_version;
    std::string channel;
    int session_timeout_ms = 0;         ///< 0 = 进程级会话不轮换
    double sample_rate = 1.0;
    int max_batch_bytes = kMaxBatchBytes;
    std::size_t max_queue_size = kMaxQueueSize;
    /// 首装自动埋 app_launch：C++ 核心层无平台存储，需平台层注入 deviceId 后置 true
    bool auto_track_launch = false;
    /// 页面浏览事件名
    std::string page_view_event = "page_view";

    /// 校验必填与契约约束，非法抛 std::invalid_argument。
    void Validate() const;
    std::string ReportUrl() const;
    std::string FlagsUrl() const;
};

/// 单次上报维度，字段与其他端 SDK 的 TrackOptions 一一对应（optional = 不发送）。
struct UbaTrackOptions {
    std::optional<std::string> event_category;
    std::optional<int64_t> user_id;
    std::optional<std::string> device_id;
    std::optional<std::string> session_id;
    std::optional<std::string> platform;
    std::optional<std::string> event_action;
    std::optional<std::string> object_type;
    std::optional<std::string> object_id;
    std::optional<std::string> object_name;
    std::optional<int64_t> duration_ms;
    std::optional<std::string> amount;
    std::optional<int64_t> quantity;
    std::optional<int32_t> score;
    std::map<std::string, double> metrics;
    std::optional<std::string> network;
    std::optional<std::string> op_result;
    std::optional<std::string> error_code;
    std::map<std::string, std::string> properties;
    std::optional<std::string> server_id;
    std::optional<int32_t> level;
    std::optional<std::string> trace_id;
};

/// 风险事件载荷（risk oneof）。risk_event_id / occur_time / report_time 服务端补全。
struct UbaRiskEvent {
    std::optional<std::string> risk_type;
    std::optional<std::string> risk_level;   // HIGH / MEDIUM / LOW
    std::optional<double> risk_score;        // 0-100
    std::optional<int64_t> rule_id;
    std::optional<std::string> rule_name;
    /// 规则上下文（任意 JSON 值：string/double/bool/嵌套 map/vector）。
    std::optional<UbaJsonValue> rule_context;
    std::optional<std::vector<std::string>> related_event_ids;
    std::optional<std::string> description;
    std::map<std::string, std::string> evidence;
    std::optional<std::string> status;
};

/// 一次 flush 的结果。
struct UbaFlushResult {
    bool success = true;
    int sent = 0;
    int dropped = 0;
    bool retryable = false;   ///< 失败是否值得补发（网络/5xx/429 为 true）
    std::string message;
};

// ─── 传输与存储接口（平台层实现：Unreal 用 FHttpModule / 文件系统注入）────────

struct UbaHttpResult {
    int status_code = 0;   ///< 0 = 网络层失败（超时/连接拒绝），可重试
    std::string body;

    bool IsOk() const { return status_code >= 200 && status_code < 300; }
};

/// HTTP 传输抽象。平台层实现（Unreal 插件提供 FHttpModule 版本），测试注入桩。
struct IUbaTransport {
    virtual ~IUbaTransport() = default;
    virtual UbaHttpResult Send(const std::string& url, const std::string& body,
                               int timeout_ms) = 0;
    virtual void Close() {}
};

/// 时钟注入点（测试用）。
struct IUbaClock {
    virtual ~IUbaClock() = default;
    virtual int64_t NowMs() = 0;
};

struct UbaRealClock : IUbaClock {
    int64_t NowMs() override;
};

// ─── 客户端 ─────────────────────────────────────────────────────────────────

/// GoWind UBA C++ 埋点客户端。线程安全。
class UbaClient {
public:
    /// @param transport 必填（平台层注入）；clock 可选（默认系统时钟）。
    UbaClient(UbaConfig config, std::shared_ptr<IUbaTransport> transport,
              std::shared_ptr<IUbaClock> clock = nullptr);
    ~UbaClient(); ///< 等价 Close()

    UbaClient(const UbaClient&) = delete;
    UbaClient& operator=(const UbaClient&) = delete;

    void Identify(int64_t user_id);       ///< 绑定用户 ID；0 = 匿名
    void ResetUser();
    void SetSuperProperties(std::map<std::string, std::string> props); ///< 合并（同名覆盖）
    void ClearSuperProperties();

    /// 上报自定义行为事件。可重试失败留在内存队列等待补发。
    void Track(const std::string& event_name,
               const std::map<std::string, std::string>& properties = {},
               const UbaTrackOptions& options = {});
    /// 上报风险事件（进入 uba_risk_events 通道）。
    void TrackRisk(const std::string& event_name, const UbaRiskEvent& risk,
                   const std::map<std::string, std::string>& properties = {},
                   const UbaTrackOptions& options = {});
    /// 页面浏览事件（含来源页与停留时长，客户端宿主场景用）。
    void TrackPageView(const std::string& screen_name,
                       const std::string& title = {},
                       const std::string& referer = {});

    bool IsCollecting() const;
    std::size_t PendingCount() const;
    std::string SessionId() const;

    /// 手动冲刷队列：分批发送直到清空或遇到失败。并发调用安全（重入直接返回）。
    UbaFlushResult Flush();
    /// 停止定时器并冲刷剩余队列。之后 Track 不再入队。
    UbaFlushResult Close();

    // AB 实验开关
    std::map<std::string, std::string> LoadFlags();   ///< 失败保留旧缓存
    std::optional<std::string> GetFlag(const std::string& key); ///< 命中自动曝光一次
    bool IsFeatureEnabled(const std::string& key);

    // 生命周期（客户端宿主接入）
    UbaFlushResult HandleBackground();
    void HandleForeground();

    // 测试与诊断
    static std::string NewUuid();
    static std::string ToRFC3339Ms(int64_t epoch_ms);
    static uint32_t Fnv1a32(const std::string& s);
    static bool SessionInSample(const std::string& session_id, double rate);

private:
    struct Impl;
    Impl* impl_;
    void Enqueue(const std::string& wire);
    UbaFlushResult SendWithRetry(const std::vector<std::string>& batch);
};

} // namespace uba
