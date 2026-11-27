// uba_core_test.cpp —— C++ 核心层测试（手写断言，零测试框架依赖）。
// 编译运行：g++ -std=c++17 -Icore core/uba_core.cpp core/uba_core_test.cpp -o uba_tests && ./uba_tests
#include "uba_core.h"

#include <iostream>
#include <string>
#include <thread>
#include <vector>

namespace {

int g_failures = 0;

#define UBA_CHECK(cond)                                                              \
    do {                                                                             \
        if (!(cond)) {                                                               \
            ++g_failures;                                                            \
            std::cerr << "FAIL @" << __LINE__ << ": " #cond << std::endl;            \
        }                                                                            \
    } while (0)

#define UBA_CHECK_CONTAINS(haystack, needle)                                         \
    do {                                                                             \
        if ((haystack).find(needle) == std::string::npos) {                          \
            ++g_failures;                                                            \
            std::cerr << "FAIL @" << __LINE__ << ": [" << (haystack)                 \
                      << "] 应包含 [" << needle << "]" << std::endl;                 \
        }                                                                            \
    } while (0)

#define UBA_CHECK_NOT_CONTAINS(haystack, needle)                                     \
    do {                                                                             \
        if ((haystack).find(needle) != std::string::npos) {                          \
            ++g_failures;                                                            \
            std::cerr << "FAIL @" << __LINE__ << ": [" << (haystack)                 \
                      << "] 不应包含 [" << needle << "]" << std::endl;               \
        }                                                                            \
    } while (0)

constexpr int kDefaultCount = 1; // 单事件断言辅助

/// 编程式假传输：记录请求 + 状态码队列。
class FakeTransport : public uba::IUbaTransport {
public:
    explicit FakeTransport(std::vector<int> statuses = {}) : statuses_(std::move(statuses)) {}

    uba::UbaHttpResult Send(const std::string& url, const std::string& body,
                            int timeout_ms) override {
        (void)timeout_ms;
        requests_.push_back({url, body});
        int status = 200;
        if (!statuses_.empty()) {
            status = statuses_.front();
            statuses_.erase(statuses_.begin());
        }
        return {status, status == 200
                            ? "{\"success\": true, \"failedCount\": 0}"
                            : "error"};
    }

    int Count() const { return static_cast<int>(requests_.size()); }
    const std::string& LastBody() const { return requests_.back().second; }
    std::size_t TotalEvents() const {
        std::size_t n = 0;
        for (const auto& [url, body] : requests_) {
            n += CountOccurrences(body, "\"eventId\"");
        }
        return n;
    }

private:
    static std::size_t CountOccurrences(const std::string& haystack, const std::string& needle) {
        std::size_t count = 0, idx = 0;
        while ((idx = haystack.find(needle, idx)) != std::string::npos) {
            ++count;
            idx += needle.size();
        }
        return count;
    }
    std::vector<std::pair<std::string, std::string>> requests_;
    std::vector<int> statuses_;
};

uba::UbaConfig MakeConfig(const std::string& url) {
    uba::UbaConfig cfg;
    cfg.app_id = "app_001";
    cfg.app_secret = "secret_001";
    cfg.endpoint = url;
    cfg.batch_size = 100;
    cfg.flush_interval_ms = 0;
    cfg.max_retries = 1;
    cfg.retry_base_delay_ms = 1;
    return cfg;
}

} // namespace

int main() {
    // ─── 工具函数 ────────────────────────────────────────────────────────────
    {
        std::string uuid = uba::UbaClient::NewUuid();
        UBA_CHECK(uuid.size() == 36);
        UBA_CHECK(uuid[14] == '4'); // version 4
        UBA_CHECK(uuid[19] == '8' || uuid[19] == '9' || uuid[19] == 'a' || uuid[19] == 'b');
        UBA_CHECK(uba::UbaClient::NewUuid() != uuid);
    }
    {
        std::string t = uba::UbaClient::ToRFC3339Ms(1774606400123);
        UBA_CHECK(t.size() == 24);
        UBA_CHECK(t[10] == 'T');
        UBA_CHECK(t.back() == 'Z');
        UBA_CHECK(t.substr(19, 4) == ".123");
    }
    {
        UBA_CHECK(uba::UbaClient::SessionInSample("s", 1.0));
        UBA_CHECK(!uba::UbaClient::SessionInSample("s", 0.0));
        bool r = uba::UbaClient::SessionInSample("fixed-session", 0.5);
        for (int i = 0; i < 10; ++i) UBA_CHECK(uba::UbaClient::SessionInSample("fixed-session", 0.5) == r);
    }

    // ─── 金样：全维度行为事件 ─────────────────────────────────────────────────
    {
        auto transport = std::make_shared<FakeTransport>();
        uba::UbaClient client(MakeConfig("http://stub"), transport);
        client.Identify(1001);
        client.SetSuperProperties({{"env", "prod"}});
        uba::UbaTrackOptions o;
        o.event_category = "order";
        o.event_action = "pay";
        o.object_type = "order";
        o.object_id = "ORD-001";
        o.object_name = "订单";
        o.duration_ms = 3500;
        o.amount = "99.90";
        o.quantity = 2;
        o.score = 88;
        o.metrics = {{"amount_num", 99.9}};
        o.network = "WiFi";
        o.op_result = "success";
        o.server_id = "s1";
        o.level = 42;
        o.trace_id = "trace-1";
        o.user_id = 1001;
        client.Track("order_paid", {{"orderId", "ORD-001"}}, o);
        UBA_CHECK(client.PendingCount() == 1);
        auto r = client.Flush();
        UBA_CHECK(r.success);
        UBA_CHECK(r.sent == kDefaultCount);
        client.Close();

        const std::string body = transport->LastBody();
        UBA_CHECK_CONTAINS(body, "\"appId\":\"app_001\"");
        UBA_CHECK_CONTAINS(body, "\"appSecret\":\"secret_001\"");
        UBA_CHECK_CONTAINS(body, "\"clientInfo\":{\"userAgent\":\"uba-cpp/");
        UBA_CHECK_CONTAINS(body, "\"eventType\":\"BEHAVIOR\"");
        UBA_CHECK_CONTAINS(body, "\"eventName\":\"order_paid\"");
        UBA_CHECK_CONTAINS(body, "\"userId\":1001");
        UBA_CHECK_CONTAINS(body, "\"properties\":{\"env\":\"prod\",\"orderId\":\"ORD-001\"}");
        // 服务端专属字段与数值标量绝不出现在顶层（数值只进 behavior，整包恰好一次）
        for (const char* key : {"tenantId", "serverTime", "\"durationMs\":3500,\"quantity\""}) {
            UBA_CHECK_NOT_CONTAINS(body, key);
        }
        for (const char* key : {"\"durationMs\":3500", "\"quantity\":2", "\"score\":88",
                                "\"level\":42", "\"serverId\":\"s1\""}) {
            std::size_t n = 0, idx = 0;
            while ((idx = body.find(key, idx)) != std::string::npos) {
                ++n;
                idx += std::string(key).size();
            }
            UBA_CHECK(n == 1);
        }
        UBA_CHECK_CONTAINS(body, "\"behavior\":{");
        UBA_CHECK_CONTAINS(body, "\"os\":\"Cpp\"");
        UBA_CHECK_CONTAINS(body, "\"metrics\":{\"amount_num\":99.9}");
    }

    // ─── 风险事件金样 ─────────────────────────────────────────────────────────
    {
        auto transport = std::make_shared<FakeTransport>();
        uba::UbaClient client(MakeConfig("http://stub"), transport);
        uba::UbaRiskEvent risk;
        risk.risk_type = "login_anomaly";
        risk.risk_level = "HIGH";
        risk.risk_score = 88.5;
        risk.rule_id = 101;
        risk.rule_context = uba::UbaJsonObject{{"threshold", uba::UbaJsonValue{int64_t{5}}}};
        risk.related_event_ids = std::vector<std::string>{"e1", "e2"};
        risk.description = "失败8次";
        risk.status = "PENDING";
        client.TrackRisk("login_failed", risk, {{"try_count", "8"}});
        client.Flush();
        client.Close();

        const std::string body = transport->LastBody();
        UBA_CHECK_CONTAINS(body, "\"eventType\":\"RISK\"");
        UBA_CHECK_NOT_CONTAINS(body, "\"behavior\":");
        UBA_CHECK_CONTAINS(body, "\"riskType\":\"login_anomaly\"");
        UBA_CHECK_CONTAINS(body, "\"riskScore\":88.5");
        UBA_CHECK_CONTAINS(body, "\"ruleId\":101");
        UBA_CHECK_CONTAINS(body, "\"ruleContext\":{\"threshold\":5}");
        UBA_CHECK_CONTAINS(body, "\"relatedEventIds\":[\"e1\",\"e2\"]");
        UBA_CHECK_CONTAINS(body, "\"status\":\"PENDING\"");
        UBA_CHECK_CONTAINS(body, "\"try_count\":\"8\"");
    }

    // ─── 401 不重试不回队 ─────────────────────────────────────────────────────
    {
        auto transport = std::make_shared<FakeTransport>(std::vector<int>{401});
        uba::UbaClient client(MakeConfig("http://stub"), transport);
        client.Track("t");
        auto r = client.Flush();
        UBA_CHECK(!r.success);
        UBA_CHECK(!r.retryable);
        UBA_CHECK(r.dropped == 1);
        UBA_CHECK(transport->Count() == 1);
        UBA_CHECK(client.PendingCount() == 0);
        client.Close();
    }

    // ─── 5xx 重试直至成功 ─────────────────────────────────────────────────────
    {
        auto transport = std::make_shared<FakeTransport>(std::vector<int>{500, 503});
        uba::UbaConfig cfg = MakeConfig("http://stub");
        cfg.max_retries = 2;
        uba::UbaClient client(cfg, transport);
        client.Track("t");
        auto r = client.Flush();
        UBA_CHECK(r.success);
        UBA_CHECK(transport->Count() == 3);
        client.Close();
    }

    // ─── 重试耗尽回队，服务恢复后补发 ─────────────────────────────────────────
    {
        auto transport = std::make_shared<FakeTransport>(std::vector<int>{500, 500});
        uba::UbaClient client(MakeConfig("http://stub"), transport);
        client.Track("offline-event", {{"k", "v"}});
        auto r = client.Flush();
        UBA_CHECK(!r.success);
        UBA_CHECK(r.retryable);
        UBA_CHECK(client.PendingCount() == 1); // 回队等待补发
        auto r2 = client.Flush();               // statuses 耗尽 → 200
        UBA_CHECK(r2.success);
        UBA_CHECK(client.PendingCount() == 0);
        client.Close();
        UBA_CHECK_CONTAINS(transport->LastBody(), "\"k\":\"v\"");
    }

    // ─── HTTP 200 部分失败按丢弃计 ────────────────────────────────────────────
    {
        class PartialTransport : public uba::IUbaTransport {
        public:
            int count = 0;
            uba::UbaHttpResult Send(const std::string&, const std::string&, int) override {
                ++count;
                return {200, "{\"success\": true, \"totalCount\": 3, \"successCount\": 1, \"failedCount\": 2}"};
            }
        };
        auto transport = std::make_shared<PartialTransport>();
        uba::UbaClient client(MakeConfig("http://stub"), transport);
        client.Track("a");
        client.Track("b");
        client.Track("c");
        auto r = client.Flush();
        UBA_CHECK(r.success);
        UBA_CHECK(r.sent == 1);
        UBA_CHECK(r.dropped == 2);
        UBA_CHECK(client.PendingCount() == 0);
        client.Close();
    }

    // ─── 并发 track 无丢失 ────────────────────────────────────────────────────
    {
        auto transport = std::make_shared<FakeTransport>();
        uba::UbaClient client(MakeConfig("http://stub"), transport);
        std::vector<std::thread> threads;
        for (int t = 0; t < 8; ++t) {
            threads.emplace_back([&client] {
                for (int i = 0; i < 25; ++i) client.Track("concurrent");
            });
        }
        for (auto& t : threads) t.join();
        auto r = client.Flush();
        UBA_CHECK(r.success);
        UBA_CHECK(transport->TotalEvents() == 200);
        client.Close();
    }

    // ─── 采样全关 → 全丢 ──────────────────────────────────────────────────────
    {
        uba::UbaConfig cfg = MakeConfig("http://stub");
        cfg.sample_rate = 0.0;
        uba::UbaClient client(cfg, std::make_shared<FakeTransport>());
        UBA_CHECK(!client.IsCollecting());
        for (int i = 0; i < 5; ++i) client.Track("t");
        UBA_CHECK(client.PendingCount() == 0);
        client.Close();
    }

    // ─── close 后拒绝入队 ─────────────────────────────────────────────────────
    {
        auto transport = std::make_shared<FakeTransport>();
        uba::UbaClient client(MakeConfig("http://stub"), transport);
        client.Track("last");
        client.Close();
        int before = transport->Count();
        client.Track("after-close");
        UBA_CHECK(client.PendingCount() == 0);
        client.Flush();
        UBA_CHECK(transport->Count() == before);
    }

    // ─── 配置校验 ─────────────────────────────────────────────────────────────
    {
        bool threw = false;
        try {
            uba::UbaConfig bad;
            bad.app_secret = "s";
            bad.endpoint = "e";
            bad.Validate(); // app_id 空
        } catch (const std::invalid_argument&) {
            threw = true;
        }
        UBA_CHECK(threw);
        threw = false;
        try {
            uba::UbaConfig bad;
            bad.app_id = "a";
            bad.app_secret = "s";
            bad.endpoint = "e";
            bad.timeout_ms = 10000;
            bad.Validate();
        } catch (const std::invalid_argument&) {
            threw = true;
        }
        UBA_CHECK(threw);
    }

    if (g_failures == 0) {
        std::cout << "ALL C++ CORE TESTS PASSED" << std::endl;
        return 0;
    }
    std::cout << g_failures << " CHECK(S) FAILED" << std::endl;
    return 1;
}
