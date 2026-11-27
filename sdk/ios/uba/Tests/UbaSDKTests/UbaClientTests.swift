import XCTest
@testable import UbaSDK

// MARK: - 可编程假传输（记录请求 + 状态码队列，与其他端 SDK 的 FakeTransport 同款）

final class StubTransport: HttpTransport {
    struct Locked<T> {
        private var value: T
        private let lock = NSLock()
        init(_ initial: T) { value = initial }
        func withLock<R>(_ body: (inout T) -> R) -> R {
            lock.lock(); defer { lock.unlock() }
            return body(&value)
        }
    }

    let requests = Locked<[(url: String, body: String)]>([])
    let statusQueue = Locked<[Int]>([])

    func send(_ url: String, _ body: String, _ timeoutMs: Int,
              completion: @escaping (HttpResult) -> Void) {
        let status: Int = statusQueue.withLock { queue -> Int in
            requests.withLock { $0.append((url, body)) }
            return queue.isEmpty ? 200 : queue.removeFirst()
        }
        let resp = status == 200
            ? "{\"success\": true, \"failedCount\": 0}"
            : "error"
        completion(HttpResult(statusCode: status, body: resp))
    }

    func close() {}

    var lastBody: String { requests.withLock { $0.last?.body ?? "" } }
    var count: Int { requests.withLock { $0.count } }
}

// MARK: - 工具单测

final class UtilTests: XCTestCase {
    func testRFC3339Millis() {
        let wire = toRFC3339Ms(Date())
        XCTAssertTrue(wire.hasSuffix("Z"))
        XCTAssertEqual(wire.count, 24)
        XCTAssertTrue(wire.contains("."))
    }

    func testSamplingDeterministic() {
        XCTAssertTrue(sessionIdInSample("s", 1.0))
        XCTAssertFalse(sessionIdInSample("s", 0.0))
        let r = sessionIdInSample("fixed-session", 0.5)
        for _ in 0..<10 {
            XCTAssertEqual(r, sessionIdInSample("fixed-session", 0.5))
        }
    }

    func testBackoff() {
        XCTAssertEqual(backoffDelayMs(1000, 0), 1000)
        XCTAssertEqual(backoffDelayMs(1000, 1), 2000)
        XCTAssertEqual(backoffDelayMs(1000, 2), 4000)
    }

    func testParseFailedCount() {
        XCTAssertEqual(UbaClient.parseFailedCount("{\"success\": true, \"failedCount\": 2}"), 2)
        XCTAssertEqual(UbaClient.parseFailedCount("{\"success\": true}"), 0)
        XCTAssertEqual(UbaClient.parseFailedCount("not json"), 0)
    }

    func testParseFlags() {
        let flags = UbaClient.parseFlags(
            "{\"success\":true,\"flags\":{\"exp\":\"red\",\"ctl\":\"control\"}}"
        )
        XCTAssertEqual(flags["exp"], "red")
        XCTAssertEqual(flags["ctl"], "control")
        XCTAssertTrue(UbaClient.parseFlags("{\"success\":true}").isEmpty)
    }

    func testParseWireList() {
        let list = UbaClient.parseWireList("[{\"a\":\"}\"},{\"b\":\"{\"}]")
        XCTAssertEqual(list.count, 2)
    }

    func testJSONEscape() {
        XCTAssertEqual(UbaJson.str("引\"号\\\n"), "\"引\\\"号\\\\\\n\"")
    }

    func testNumericScalarsOnlyInBehavior() {
        let event = ReportEvent(
            eventType: "BEHAVIOR", eventId: "e1", eventName: "level_pass",
            eventTime: "2026-03-27T12:00:00.123Z", userId: 1001,
            deviceId: "d1", sessionId: "s1", platform: "ios",
            eventCategory: "game", properties: ["scene": "x"], metrics: ["combo": 3],
            traceId: nil, eventAction: "pass", objectType: "level", objectId: "42",
            objectName: nil, amount: "0.99", opResult: "success", errorCode: nil,
            behavior: BehaviorPayload(
                eventAction: "pass", objectType: "level", objectId: "42",
                objectName: nil, sessionSeq: nil, os: "iOS", appVersion: "1.0",
                channel: nil, network: "WiFi", durationMs: 3500, amount: "0.99",
                quantity: 2, score: 88, metrics: ["combo": 3], opResult: "success",
                errorCode: nil, serverId: "s1", level: 42
            ),
            risk: nil
        )
        let wire = UbaJson.eventToWire(event)
        XCTAssertTrue(UbaJson.envelope("a", "s", "ua", [wire]).contains("\"events\":[{"))
        // 数值标量恰好出现一次（只进 behavior）
        for key in ["\"durationMs\"", "\"quantity\"", "\"score\"", "\"level\""] {
            XCTAssertEqual(wire.components(separatedBy: key).count - 1, 1, key)
        }
        // 服务端专属字段不发
        XCTAssertFalse(wire.contains("tenantId"))
        XCTAssertFalse(wire.contains("serverTime"))
    }
}

// MARK: - 客户端行为

final class UbaClientTests: XCTestCase {
    private var transport: StubTransport!
    private var store: InMemoryStore!

    override func setUp() {
        super.setUp()
        transport = StubTransport()
        store = InMemoryStore()
    }

    override func tearDown() {
        transport = nil
        store = nil
        super.tearDown()
    }

    private func makeConfig() -> UbaConfig {
        UbaConfig(
            appId: "app_001",
            appSecret: "secret_001",
            endpoint: "http://collector.example.com",
            batchSize: 100,
            flushIntervalMs: 0,
            maxRetries: 1,
            retryBaseDelayMs: 1
        )
    }

    func testGoldenBehaviorEvent() {
        let client = UbaClient(config: makeConfig(), store: store, transport: transport)
        client.identify(1001)
        client.setSuperProperties(["env": "prod"])
        client.track("order_paid",
                     properties: ["orderId": "ORD-001"],
                     options: TrackOptions(
                        eventCategory: "order", eventAction: "pay",
                        objectType: "order", objectId: "ORD-001", objectName: "订单",
                        durationMs: 3500, amount: "99.90", quantity: 2, score: 88,
                        metrics: ["amount_num": 99.9], network: "WiFi",
                        opResult: "success", serverId: "s1", level: 42, traceId: "trace-1"
                     ))
        let r = client.flush()
        XCTAssertTrue(r.success)
        XCTAssertEqual(r.sent, 1)
        client.close()

        let body = transport.lastBody
        XCTAssertTrue(body.contains("\"appId\":\"app_001\""))
        XCTAssertTrue(body.contains("\"appSecret\":\"secret_001\""))
        XCTAssertTrue(body.contains("\"clientInfo\":{\"userAgent\":\"uba-swift/"))
        XCTAssertTrue(body.contains("\"eventType\":\"BEHAVIOR\""))
        XCTAssertTrue(body.contains("\"eventName\":\"order_paid\""))
        XCTAssertTrue(body.contains("\"userId\":1001"))
        XCTAssertTrue(body.contains("\"env\":\"prod\""))
        XCTAssertTrue(body.contains("\"platform\":\"ios\""))
        XCTAssertTrue(RegexHelper.rfc3339Matches(body))
        for key in ["\"durationMs\"", "\"quantity\"", "\"score\"", "\"level\""] {
            XCTAssertEqual(body.components(separatedBy: key).count - 1, 1, key)
        }
        XCTAssertFalse(body.contains("tenantId"))
        XCTAssertFalse(body.contains("serverTime"))
    }

    func testRiskEventGolden() {
        let client = UbaClient(config: makeConfig(), store: store, transport: transport)
        client.trackRisk("login_failed", RiskEvent(
            riskType: "login_anomaly",
            riskLevel: "HIGH",
            riskScore: 88.5,
            ruleId: 101,
            ruleContext: ["threshold": 5],
            relatedEventIds: ["e1", "e2"],
            description: "失败8次",
            status: "PENDING"
        ), properties: ["try_count": "8"])
        client.flush()
        client.close()

        let body = transport.lastBody
        XCTAssertTrue(body.contains("\"eventType\":\"RISK\""))
        XCTAssertFalse(body.contains("\"behavior\":"))
        XCTAssertTrue(body.contains("\"riskType\":\"login_anomaly\""))
        XCTAssertTrue(body.contains("\"riskScore\":88.5"))
        XCTAssertTrue(body.contains("\"ruleId\":101"))
        XCTAssertTrue(body.contains("\"ruleContext\":{\"threshold\":5}"))
        XCTAssertTrue(body.contains("\"status\":\"PENDING\""))
    }

    func testNoRetryOn401() {
        transport.statusQueue.withLock { $0.append(401) }
        let client = UbaClient(config: makeConfig(), store: store, transport: transport)
        client.track("t")
        let r = client.flush()
        XCTAssertFalse(r.success)
        XCTAssertFalse(r.retryable)
        XCTAssertEqual(r.dropped, 1)
        XCTAssertEqual(transport.count, 1)
        XCTAssertEqual(client.pendingCount(), 0)
        client.close()
    }

    func testRetryExhaustedRestoresFromStore() {
        transport.statusQueue.withLock {
            $0.append(500)
            $0.append(500)
        }
        let client = UbaClient(config: makeConfig(), store: store, transport: transport)
        client.track("offline-event", properties: ["k": "v"])
        let r = client.flush()
        XCTAssertFalse(r.success)
        XCTAssertTrue(r.retryable)
        XCTAssertEqual(client.pendingCount(), 1)
        XCTAssertTrue(store.getString(offlineQueueKey)?.contains("offline-event") == true)
        client.close()

        // 服务恢复：新实例从离线盘恢复补发
        let client2 = UbaClient(config: makeConfig(), store: store, transport: transport)
        XCTAssertEqual(client2.pendingCount(), 1)
        let r2 = client2.flush()
        XCTAssertTrue(r2.success)
        XCTAssertEqual(client2.pendingCount(), 0)
        XCTAssertNil(store.getString(offlineQueueKey))
        client2.close()
        XCTAssertTrue(transport.lastBody.contains("\"k\":\"v\""))
    }

    func testConsentGating() {
        let client = UbaClient(config: UbaConfig(
            appId: "a", appSecret: "s", endpoint: "http://collector.example.com",
            requireConsent: true
        ), store: store, transport: transport)
        XCTAssertFalse(client.isCollecting())
        client.track("t")
        XCTAssertEqual(client.pendingCount(), 0)
        client.setConsent(true)
        XCTAssertTrue(client.isCollecting())
        client.track("t2")
        XCTAssertEqual(client.pendingCount(), 1)
        client.close()
    }

    func testClosedClientRejects() {
        let client = UbaClient(config: makeConfig(), store: store, transport: transport)
        client.track("last")
        client.close()
        let before = transport.count
        client.track("after-close")
        XCTAssertEqual(client.pendingCount(), 0)
        _ = client.flush()
        XCTAssertEqual(transport.count, before)
    }

    func testConfigPreconditions() {
        XCTAssertThrowsError(try {
            _ = UbaConfig(appId: "", appSecret: "s", endpoint: "e")
        }())
        XCTAssertThrowsError(try {
            _ = UbaConfig(appId: "a", appSecret: "s", endpoint: "e", timeoutMs: 10000)
        }())
    }
}

/// RFC3339 毫秒格式形状断言（不依赖具体时刻，避免时区差异）。
private enum RegexHelper {
    static func rfc3339Matches(_ body: String) -> Bool {
        guard let range = body.range(of: "\"eventTime\":\"") else { return false }
        let candidate = String(body[range.upperBound...].prefix(24))
        let pattern = "^\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2}\\.\\d{3}Z$"
        return candidate.range(of: pattern, options: .regularExpression) != nil
    }
}
