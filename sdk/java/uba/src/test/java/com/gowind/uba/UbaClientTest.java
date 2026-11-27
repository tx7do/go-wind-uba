package com.gowind.uba;

import com.sun.net.httpserver.HttpServer;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.concurrent.ConcurrentLinkedQueue;
import java.util.concurrent.atomic.AtomicInteger;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNull;
import static org.junit.jupiter.api.Assertions.assertTrue;

/** 客户端契约金样与重试语义（对照 backend/app/collector/service/README.md 样例）。 */
class UbaClientTest {

    private HttpServer server;
    private final ConcurrentLinkedQueue<String> bodies = new ConcurrentLinkedQueue<>();
    /** 编程式状态码队列；空则 200。 */
    private final List<Integer> statusQueue = new ArrayList<>();
    private final AtomicInteger seq = new AtomicInteger();
    private final Object lock = new Object();

    @BeforeEach
    void setUp() throws IOException {
        server = HttpServer.create(new InetSocketAddress(0), 0);
        server.createContext("/", exchange -> {
            String body = new String(exchange.getRequestBody().readAllBytes(), StandardCharsets.UTF_8);
            bodies.add(body);
            int n = seq.incrementAndGet();
            int status;
            synchronized (lock) {
                status = statusQueue.isEmpty() ? 200 : statusQueue.remove(0);
            }
            byte[] resp;
            if (status == 200) {
                resp = ("{\"success\": true, \"failedCount\": 0, \"n\": " + n + "}")
                        .getBytes(StandardCharsets.UTF_8);
                exchange.sendResponseHeaders(200, resp.length);
            } else {
                resp = "error".getBytes(StandardCharsets.UTF_8);
                exchange.sendResponseHeaders(status, resp.length);
            }
            try (OutputStream os = exchange.getResponseBody()) {
                os.write(resp);
            }
        });
        server.start();
    }

    @AfterEach
    void tearDown() {
        server.stop(0);
    }

    private UbaConfig config() {
        return new UbaConfig()
                .appId("app_001")
                .appSecret("secret_001")
                .endpoint("http://localhost:" + server.getAddress().getPort())
                .batchSize(100)
                .flushInterval(java.time.Duration.ZERO)
                .maxRetries(1)
                .retryBaseDelay(java.time.Duration.ofMillis(1));
    }

    private String lastBody() {
        return bodies.stream().reduce((a, b) -> b).orElseThrow();
    }

    @Test
    void goldenBehaviorEvent() {
        UbaClient client = new UbaClient(config());
        client.identify(1001);
        client.setSuperProperties(Map.of("env", "prod"));
        Map<String, String> props = new java.util.LinkedHashMap<>();
        props.put("orderId", "ORD-001");
        client.track("order_paid", props, new TrackOptions()
                .eventCategory("order")
                .eventAction("pay")
                .objectType("order")
                .objectId("ORD-001")
                .objectName("订单")
                .durationMs(3500)
                .amount("99.90")
                .quantity(2)
                .score(88)
                .metrics(Map.of("amount_num", 99.9))
                .serverId("s1")
                .level(42)
                .traceId("trace-1")
                .network("WiFi")
                .opResult("success"));
        FlushResult r = client.flush();
        assertTrue(r.success);
        assertEquals(1, r.sent);
        client.close();

        String body = lastBody();
        assertTrue(body.contains("\"appId\":\"app_001\""));
        assertTrue(body.contains("\"appSecret\":\"secret_001\""));
        assertTrue(body.contains("\"clientInfo\":{\"userAgent\":"));
        // 必填四件套 + oneof
        assertTrue(body.contains("\"eventType\":\"BEHAVIOR\""));
        assertTrue(body.contains("\"eventName\":\"order_paid\""));
        assertTrue(body.contains("\"eventTime\":\"20"));
        // 数值标量只进 behavior oneof：每个数值键在整包 JSON 里恰好出现一次
        // （顶层没有这些键，behavior 内一次；metrics 属 map 键不算）
        assertEquals(1, countOccurrences(body, "\"durationMs\""), "durationMs 应只在 behavior 内");
        assertEquals(1, countOccurrences(body, "\"quantity\""));
        assertEquals(1, countOccurrences(body, "\"score\""));
        assertEquals(1, countOccurrences(body, "\"level\""));
        assertFalse(body.contains("\"tenantId\""));
        // 服务端专属字段不发
        assertFalse(body.contains("tenantId"));
        assertFalse(body.contains("serverTime"));
        assertFalse(body.contains("\"ip\":"));
        // userId 是数字不是字符串
        assertTrue(body.contains("\"userId\":1001"));
        // super 属性合并
        assertTrue(body.contains("\"env\":\"prod\""));
    }

    @Test
    void riskEventGolden() {
        UbaClient client = new UbaClient(config());
        client.trackRisk("login_failed", new RiskEvent()
                .riskType("login_anomaly")
                .riskLevel("HIGH")
                .riskScore(88.5)
                .ruleId(101)
                .ruleContext(Map.of("threshold", 5))
                .relatedEventIds(List.of("e1", "e2"))
                .description("失败8次")
                .status("PENDING"), Map.of("try_count", "8"), null);
        client.flush();
        client.close();

        String body = lastBody();
        assertTrue(body.contains("\"eventType\":\"RISK\""));
        assertFalse(body.contains("\"behavior\":"));
        assertTrue(body.contains("\"risk\":{\"riskType\":\"login_anomaly\""));
        assertTrue(body.contains("\"riskScore\":88.5"));
        assertTrue(body.contains("\"ruleId\":101"));
        assertTrue(body.contains("\"ruleContext\":{\"threshold\":5}"));
        assertTrue(body.contains("\"status\":\"PENDING\""));
    }

    @Test
    void noRetryOn401() {
        synchronized (lock) {
            statusQueue.add(401);
        }
        UbaClient client = new UbaClient(config());
        client.track("t", null, null);
        FlushResult r = client.flush();
        assertFalse(r.success);
        assertFalse(r.retryable);
        assertEquals(1, r.dropped);
        assertEquals(1, bodies.size()); // 只打了一次
        assertEquals(0, client.pendingCount()); // 确定性失败不回队
        client.close();
    }

    @Test
    void retryOn5xxUntilSuccess() {
        synchronized (lock) {
            statusQueue.add(500);
            statusQueue.add(503);
        }
        UbaConfig cfg = config().maxRetries(2); // 3 次尝试
        UbaClient client = new UbaClient(cfg);
        client.track("t", null, null);
        FlushResult r = client.flush();
        assertTrue(r.success);
        assertEquals(3, bodies.size());
        client.close();
    }

    @Test
    void retryExhaustedStaysQueued() {
        synchronized (lock) {
            statusQueue.add(500);
            statusQueue.add(500);
        }
        UbaClient client = new UbaClient(config());
        client.track("t", null, null);
        FlushResult r = client.flush();
        assertFalse(r.success);
        assertTrue(r.retryable);
        assertEquals(1, client.pendingCount()); // 回队等待补发

        // 服务恢复（statusQueue 已空 → 200）
        FlushResult r2 = client.flush();
        assertTrue(r2.success);
        assertEquals(0, client.pendingCount());
        client.close();
    }

    @Test
    void partialFailureOnHttp200() {
        server.stop(0);
        try {
            server = HttpServer.create(new InetSocketAddress(0), 0);
            AtomicInteger count = new AtomicInteger();
            server.createContext("/", exchange -> {
                byte[] resp = "{\"success\": true, \"totalCount\": 3, \"successCount\": 1, \"failedCount\": 2}"
                        .getBytes(StandardCharsets.UTF_8);
                exchange.sendResponseHeaders(200, resp.length);
                try (OutputStream os = exchange.getResponseBody()) {
                    os.write(resp);
                }
                count.incrementAndGet();
            });
            server.start();

            UbaClient client = new UbaClient(new UbaConfig()
                    .appId("app_001").appSecret("secret_001")
                    .endpoint("http://localhost:" + server.getAddress().getPort())
                    .batchSize(100).flushInterval(java.time.Duration.ZERO)
                    .maxRetries(1).retryBaseDelay(java.time.Duration.ofMillis(1)));
            client.track("a", null, null);
            client.track("b", null, null);
            client.track("c", null, null);
            FlushResult r = client.flush();
            assertTrue(r.success); // HTTP 200 部分失败仍算传输成功
            assertEquals(1, r.sent);
            assertEquals(2, r.dropped);
            assertEquals(0, client.pendingCount()); // 服务端明确拒绝的不回队
            client.close();
        } catch (IOException e) {
            throw new RuntimeException(e);
        }
    }

    @Test
    void flagsLoadAndExposure() {
        UbaClient client = new UbaClient(config());
        client.identify(7);
        // flags 请求走同一路径前缀；statusQueue 为空 → 200，但 body 是 report 样式…
        // 直接换一个带 flags 响应的 handler
        server.stop(0);
        try {
            server = HttpServer.create(new InetSocketAddress(0), 0);
            server.createContext("/", exchange -> {
                String path = exchange.getRequestURI().getPath();
                String reqBody = new String(exchange.getRequestBody().readAllBytes(), StandardCharsets.UTF_8);
                bodies.add(reqBody);
                byte[] resp;
                if (path.endsWith("/uba/v1/flags")) {
                    resp = "{\"flags\":{\"exp\":\"red\",\"ctl\":\"control\"}}".getBytes(StandardCharsets.UTF_8);
                } else {
                    resp = "{\"success\": true, \"failedCount\": 0}".getBytes(StandardCharsets.UTF_8);
                }
                exchange.sendResponseHeaders(200, resp.length);
                try (OutputStream os = exchange.getResponseBody()) {
                    os.write(resp);
                }
            });
            server.start();
        } catch (IOException e) {
            throw new RuntimeException(e);
        }

        UbaClient c2 = new UbaClient(new UbaConfig()
                .appId("app_001").appSecret("secret_001")
                .endpoint("http://localhost:" + server.getAddress().getPort())
                .batchSize(100).flushInterval(java.time.Duration.ZERO)
                .maxRetries(1).retryBaseDelay(java.time.Duration.ofMillis(1)));
        c2.identify(7);
        try {
            Map<String, String> flags = c2.loadFlags();
            assertEquals("red", flags.get("exp"));
            assertEquals("red", c2.getFlag("exp"));
            assertFalse(c2.isFeatureEnabled("ctl"));
            assertNull(c2.getFlag("missing"));
            c2.flush();
            String body = lastBody();
            assertTrue(body.contains("$feature_flag"));
            assertTrue(body.contains("\"flag_key\":\"exp\""));
            // 同会话去重
            c2.getFlag("exp");
            assertEquals(0, c2.pendingCount());
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            throw new RuntimeException(e);
        }
        c2.close();
    }

    @Test
    void closedClientRejects() {
        UbaClient client = new UbaClient(config());
        client.track("last", null, null);
        client.close(); // close 会冲刷剩余队列
        int before = bodies.size();
        client.track("after-close", null, null);
        assertEquals(0, client.pendingCount());
        client.flush();
        assertEquals(before, bodies.size());
    }

    @Test
    void trackOptionsOverrideIdentity() {
        UbaClient client = new UbaClient(config());
        client.identify(1);
        client.track("t", null, new TrackOptions()
                .userId(999).deviceId("custom").sessionId("custom-sess").platform("game"));
        client.flush();
        client.close();

        String body = lastBody();
        assertTrue(body.contains("\"userId\":999"));
        assertTrue(body.contains("\"deviceId\":\"custom\""));
        assertTrue(body.contains("\"sessionId\":\"custom-sess\""));
        assertTrue(body.contains("\"platform\":\"game\""));
    }

    @Test
    void sessionRotationOnTimeout() throws Exception {
        UbaConfig cfg = config().sessionTimeout(java.time.Duration.ofMillis(30));
        UbaClient client = new UbaClient(cfg);
        client.track("a", null, null);
        client.flush();
        String sess1 = extractField(lastBody(), "sessionId");

        Thread.sleep(40);
        client.track("b", null, null);
        client.flush();
        client.close();
        String sess2 = extractField(lastBody(), "sessionId");
        assertFalse(sess1.equals(sess2), "超时后应轮换会话: " + sess1 + " vs " + sess2);
    }

    private static String extractField(String json, String field) {
        int idx = json.indexOf("\"" + field + "\":\"");
        if (idx < 0) return "";
        int start = idx + field.length() + 4;
        int end = json.indexOf('"', start);
        return json.substring(start, end);
    }

    private static int countOccurrences(String haystack, String needle) {
        int count = 0;
        int idx = 0;
        while ((idx = haystack.indexOf(needle, idx)) >= 0) {
            count++;
            idx += needle.length();
        }
        return count;
    }
}
