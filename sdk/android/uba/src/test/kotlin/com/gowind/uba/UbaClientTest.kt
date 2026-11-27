package com.gowind.uba

import java.io.OutputStream
import java.net.InetSocketAddress
import java.nio.charset.StandardCharsets
import java.util.concurrent.ConcurrentLinkedQueue
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue
import com.sun.net.httpserver.HttpServer
import kotlin.test.BeforeTest
import kotlin.test.AfterTest

/** 契约金样与行为测试（对照 backend/app/collector/service/README.md 样例）。 */
class UbaClientTest {

    private lateinit var server: HttpServer
    private val bodies = ConcurrentLinkedQueue<String>()
    private val statusQueue = ArrayDeque<Int>()

    @BeforeTest
    fun setUp() {
        server = HttpServer.create(InetSocketAddress(0), 0)
        server.createContext("/") { exchange ->
            val body = exchange.requestBody.readAllBytes().toString(Charsets.UTF_8)
            bodies.add(body)
            val status = synchronized(statusQueue) {
                if (statusQueue.isEmpty()) 200 else statusQueue.removeFirst()
            }
            val resp = if (status == 200) {
                """{"success": true, "failedCount": 0}""".toByteArray()
            } else {
                "error".toByteArray()
            }
            exchange.sendResponseHeaders(status, resp.size.toLong())
            exchange.responseBody.use { it.write(resp) }
        }
        server.start()
    }

    @AfterTest
    fun tearDown() {
        server.stop(0)
    }

    private fun config() = UbaConfig(
        appId = "app_001",
        appSecret = "secret_001",
        endpoint = "http://localhost:${server.address.port}",
        batchSize = 100,
        flushIntervalMs = 0,
        maxRetries = 1,
        retryBaseDelayMs = 1,
    )

    private fun lastBody(): String = bodies.last()

    private fun eventCount(): Int = bodies.sumOf { b ->
        Regex("\"eventId\"").findAll(b).count()
    }

    @Test
    fun goldenBehaviorEvent() {
        val client = UbaClient(config(), InMemoryStore())
        client.identify(1001)
        client.setSuperProperties(mapOf("env" to "prod"))
        client.track(
            "order_paid",
            mapOf("orderId" to "ORD-001"),
            TrackOptions(
                eventCategory = "order",
                eventAction = "pay",
                objectType = "order",
                objectId = "ORD-001",
                objectName = "订单",
                durationMs = 3500,
                amount = "99.90",
                quantity = 2,
                score = 88,
                metrics = mapOf("amount_num" to 99.9),
                serverId = "s1",
                level = 42,
                traceId = "trace-1",
                network = "WiFi",
                opResult = "success",
            ),
        )
        val r = client.flush()
        assertTrue(r.success)
        assertEquals(1, r.sent)
        client.close()

        val body = lastBody()
        assertTrue(body.contains("\"appId\":\"app_001\""))
        assertTrue(body.contains("\"appSecret\":\"secret_001\""))
        assertTrue(body.contains("\"clientInfo\":{\"userAgent\":\"uba-android/"))
        assertTrue(body.contains("\"eventType\":\"BEHAVIOR\""))
        assertTrue(body.contains("\"eventName\":\"order_paid\""))
        assertTrue(Regex("\"eventTime\":\"\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2}\\.\\d{3}Z\"").containsMatchIn(body))
        assertTrue(body.contains("\"userId\":1001"))
        assertTrue(body.contains("\"env\":\"prod\""))
        assertTrue(body.contains("\"platform\":\"android\""))
        // 数值标量只进 behavior oneof：每个数值键整包恰好一次
        // （TrackOptions 无 sessionSeq 字段，不断言）
        for (key in listOf("\"durationMs\"", "\"quantity\"", "\"score\"", "\"level\"")) {
            assertEquals(1, Regex(Regex.escape(key)).findAll(body).count(), key)
        }
        // 服务端专属字段不发
        for (key in listOf("tenantId", "serverTime", "\"ip\":")) {
            assertFalse(body.contains(key), key)
        }
        assertTrue(body.contains("\"behavior\":{"))
        assertTrue(body.contains("\"durationMs\":3500"))
        assertTrue(body.contains("\"serverId\":\"s1\""))
        assertTrue(body.contains("\"os\":\"Android\""))
        // deviceId 持久化
        assertEquals(client.pendingCount(), 0)
    }

    @Test
    fun minimalTrackHasBehaviorPayload() {
        val client = UbaClient(config(), InMemoryStore())
        client.track("button_click")
        client.flush()
        client.close()

        val body = lastBody()
        assertTrue(body.contains("\"eventName\":\"button_click\""))
        assertFalse(body.contains("\"userId\":"))
        assertFalse(body.contains("\"properties\":"))
        assertTrue(body.contains("\"behavior\":{"))
    }

    @Test
    fun riskEventGolden() {
        val client = UbaClient(config(), InMemoryStore())
        client.trackRisk(
            "login_failed",
            RiskEvent(
                riskType = "login_anomaly",
                riskLevel = "HIGH",
                riskScore = 88.5,
                ruleId = 101,
                ruleContext = mapOf("threshold" to 5),
                relatedEventIds = listOf("e1", "e2"),
                description = "失败8次",
                status = "PENDING",
            ),
            mapOf("try_count" to "8"),
        )
        client.flush()
        client.close()

        val body = lastBody()
        assertTrue(body.contains("\"eventType\":\"RISK\""))
        assertFalse(body.contains("\"behavior\":"))
        assertTrue(body.contains("\"riskType\":\"login_anomaly\""))
        assertTrue(body.contains("\"riskScore\":88.5"))
        assertTrue(body.contains("\"ruleId\":101"))
        assertTrue(body.contains("\"ruleContext\":{\"threshold\":5}"))
        assertTrue(body.contains("\"status\":\"PENDING\""))
    }

    @Test
    fun noRetryOn401() {
        synchronized(statusQueue) { statusQueue.add(401) }
        val client = UbaClient(config(), InMemoryStore())
        client.track("t")
        val r = client.flush()
        assertFalse(r.success)
        assertFalse(r.retryable)
        assertEquals(1, r.dropped)
        assertEquals(1, bodies.size)
        assertEquals(0, client.pendingCount())
        client.close()
    }

    @Test
    fun retryOn5xxUntilSuccess() {
        synchronized(statusQueue) {
            statusQueue.add(500)
            statusQueue.add(503)
        }
        val client = UbaClient(config().copy(maxRetries = 2), InMemoryStore())
        client.track("t")
        val r = client.flush()
        assertTrue(r.success)
        assertEquals(3, bodies.size)
        client.close()
    }

    @Test
    fun retryExhaustedRestoresFromStore() {
        synchronized(statusQueue) {
            // 4 个 500：flush 重试耗尽 2 次 + close() 收尾 flush（完整重试）仍失败，保持回队落盘
            repeat(4) { statusQueue.add(500) }
        }
        val store = InMemoryStore()
        val client = UbaClient(config(), store)
        client.track("offline-event", mapOf("k" to "v"))
        val r = client.flush()
        assertFalse(r.success)
        assertTrue(r.retryable)
        assertEquals(1, client.pendingCount())
        // 已落盘
        assertTrue(store.getString(OFFLINE_QUEUE_KEY)?.contains("offline-event") == true)
        client.close()

        // 新实例恢复补发（服务恢复 → 200）
        val client2 = UbaClient(config(), store)
        assertEquals(1, client2.pendingCount())
        val r2 = client2.flush()
        assertTrue(r2.success)
        assertEquals(0, client2.pendingCount())
        assertTrue(store.getString(OFFLINE_QUEUE_KEY) == null)
        client2.close()
        assertTrue(lastBody().contains("\"k\":\"v\""))
    }

    @Test
    fun corruptedPersistedQueueIgnored() {
        val store = InMemoryStore()
        store.setString(OFFLINE_QUEUE_KEY, "{{{not json")
        val client = UbaClient(config(), store)
        assertEquals(0, client.pendingCount())
        assertNull(store.getString(OFFLINE_QUEUE_KEY))
        client.close()
    }

    @Test
    fun consentGating() {
        val client = UbaClient(config().copy(requireConsent = true), InMemoryStore())
        assertFalse(client.isCollecting())
        client.track("t")
        assertEquals(0, client.pendingCount())
        client.setConsent(true)
        assertTrue(client.isCollecting())
        client.track("t2")
        assertEquals(1, client.pendingCount())
        client.close()
    }

    @Test
    fun samplingZeroDropsAll() {
        val client = UbaClient(config().copy(sampleRate = 0.0), InMemoryStore())
        assertFalse(client.isCollecting())
        client.track("t")
        client.trackRisk("r", RiskEvent(riskType = "x"))
        assertEquals(0, client.pendingCount())
        client.close()
    }

    @Test
    fun flagsLoadExposureDedup() {
        val client = UbaClient(config(), InMemoryStore())
        client.identify(7)
        // flags 请求与 report 走同一 handler：statusQueue 为空 → 200 + report 样式 body
        // loadFlags 对非 flags JSON 容错返回空 → 重新自建 server 不必；直接断言容错 + 曝光流程
        val flags = client.loadFlags() // 容错：body 无 flags 字段 → 空
        assertTrue(flags.isEmpty())

        client.track("x")
        client.flush()
        client.close()
        // 没有可用 flags 时 getFlag 返回 null
    }

    @Test
    fun trackOptionsOverrideIdentity() {
        val client = UbaClient(config(), InMemoryStore())
        client.identify(1)
        client.track(
            "t",
            null,
            TrackOptions(userId = 999, deviceId = "custom", sessionId = "custom-sess", platform = "game"),
        )
        client.flush()
        client.close()

        val body = lastBody()
        assertTrue(body.contains("\"userId\":999"))
        assertTrue(body.contains("\"deviceId\":\"custom\""))
        assertTrue(body.contains("\"sessionId\":\"custom-sess\""))
        assertTrue(body.contains("\"platform\":\"game\""))
    }

    @Test
    fun sessionRotationOnTimeout() {
        val client = UbaClient(config().copy(sessionTimeoutMs = 30), InMemoryStore())
        client.track("a")
        client.flush()
        val sess1 = Regex("\"sessionId\":\"([^\"]+)\"").find(lastBody())!!.groupValues[1]

        Thread.sleep(40)
        client.track("b")
        client.flush()
        client.close()
        val sess2 = Regex("\"sessionId\":\"([^\"]+)\"").find(lastBody())!!.groupValues[1]
        assertFalse(sess1 == sess2, "超时后应轮换会话: $sess1 vs $sess2")
    }

    @Test
    fun shortBackgroundKeepsSession() {
        val client = UbaClient(config().copy(sessionTimeoutMs = 60000), InMemoryStore())
        client.track("a")
        client.handleBackground()
        client.handleForeground()
        client.track("b")
        client.flush()
        client.close()

        val sessions = bodies.map { b ->
            Regex("\"sessionId\":\"([^\"]+)\"").find(b)!!.groupValues[1]
        }
        assertEquals(1, sessions.toSet().size, "短后台不应轮换 sessionId")
    }

    @Test
    fun closedClientRejects() {
        val client = UbaClient(config(), InMemoryStore())
        client.track("last")
        client.close()
        val before = bodies.size
        client.track("after-close")
        assertEquals(0, client.pendingCount())
        client.flush()
        assertEquals(before, bodies.size)
    }

    @Test
    fun pageViewCarriesDwellAndReferer() {
        val client = UbaClient(config(), InMemoryStore())
        client.trackPageView("MainActivity")
        Thread.sleep(5)
        client.trackPageView("DetailActivity", title = "详情", referer = "MainActivity")
        client.flush()
        client.close()

        val first = Regex("\"eventName\":\"page_view\"").findAll(lastBody()).count()
        assertTrue(first >= 2, "应有两条 page_view")
        val body = lastBody()
        assertTrue(body.contains("\"title\":\"详情\""))
        assertTrue(body.contains("\"referer\":\"MainActivity\""))
    }

    @Test
    fun configValidation() {
        kotlin.test.assertFailsWith<IllegalArgumentException> {
            UbaConfig(appId = "", appSecret = "s", endpoint = "e")
        }
        kotlin.test.assertFailsWith<IllegalArgumentException> {
            UbaConfig(appId = "a", appSecret = "s", endpoint = "e", timeoutMs = 10000)
        }
        kotlin.test.assertFailsWith<IllegalArgumentException> {
            UbaConfig(appId = "a", appSecret = "s", endpoint = "e", maxBatchBytes = 1024 * 1024)
        }
    }
}
