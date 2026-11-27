package com.gowind.uba;

import java.net.InetAddress;
import java.time.Instant;
import java.time.ZoneOffset;
import java.time.format.DateTimeFormatter;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.TreeSet;
import java.util.concurrent.Executors;
import java.util.concurrent.ScheduledExecutorService;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicLong;
import java.util.concurrent.locks.ReentrantLock;

/**
 * GoWind UBA Java 服务端埋点客户端。线程安全。
 *
 * <pre>{@code
 * UbaClient client = new UbaClient(new UbaConfig()
 *     .appId("your_app_id")
 *     .appSecret("your_app_secret")
 *     .endpoint("http://localhost:5700"));
 * client.identify(1001);
 * client.track("order_paid", TrackOptions.p("orderId", "ORD-001"),
 *     new TrackOptions().objectType("order").amount("99.90"));
 * client.close(); // 冲刷剩余队列并停止定时器
 * }</pre>
 *
 * 契约铁律（与其他端 SDK 一致）：camelCase 对齐 proto json_name；
 * tenantId/serverTime/ip 服务端补全不发送；数值标量只进 behavior oneof；
 * 4xx（除 429）不重试；单批请求体压在 1MiB 内；超时必须小于服务端 10s。
 */
public final class UbaClient {

    /** SDK 版本，用于 User-Agent；与 pom 版本保持一致。 */
    public static final String SDK_VERSION = "1.0.0";

    private static final DateTimeFormatter RFC3339_MS =
            DateTimeFormatter.ofPattern("yyyy-MM-dd'T'HH:mm:ss.SSS'Z'").withZone(ZoneOffset.UTC);

    private final UbaConfig config;
    private final HttpTransport transport;

    private final ReentrantLock lock = new ReentrantLock();
    private final List<Map<String, Object>> queue = new ArrayList<>();
    private boolean flushing;
    private boolean closed;

    private final AtomicLong userId = new AtomicLong(0);
    private final Map<String, String> superProps = new LinkedHashMap<>();
    private final Map<String, String> flags = new LinkedHashMap<>();
    private final TreeSet<String> exposedFlags = new TreeSet<>();
    private String exposedSessionId;
    private String sessionId = newUuid();
    private Instant lastEventAt;

    private final ScheduledExecutorService timer;
    private final String userAgent;
    private final String deviceId;
    private final String osName;

    /** 创建并启动客户端（定时 flush 随 {@link #close()} 停止）。 */
    public UbaClient(UbaConfig config) {
        this(config, null);
    }

    /** 测试/定制入口：注入自定义传输实现。 */
    public UbaClient(UbaConfig config, HttpTransport transport) {
        config.validate();
        this.config = config;
        HttpTransport t = transport;
        if (t == null) t = new HttpTransport.JdkHttpTransport();
        this.transport = t;
        this.deviceId = config.deviceId().isEmpty() ? defaultHostname() : config.deviceId();
        this.osName = System.getProperty("os.name", "unknown").toLowerCase();
        this.userAgent = "uba-java/" + SDK_VERSION + " (" + osName + ")";
        this.timer = config.flushInterval().isZero()
                ? null
                : Executors.newSingleThreadScheduledExecutor(r -> {
                    Thread thread = new Thread(r, "uba-flush");
                    thread.setDaemon(true);
                    return thread;
                });
        if (timer != null) {
            long millis = config.flushInterval().toMillis();
            timer.scheduleWithFixedDelay(this::flushQuietly, millis, millis, TimeUnit.MILLISECONDS);
        }
    }

    /** 绑定用户 ID（服务端事件源里的用户）；0 = 匿名。 */
    public void identify(long id) {
        userId.set(id);
    }

    /** 清空用户绑定。 */
    public void resetUser() {
        userId.set(0);
    }

    /** 设置全局公共属性：合并进之后每个事件（同名覆盖）。 */
    public void setSuperProperties(Map<String, String> props) {
        lock.lock();
        try {
            superProps.putAll(props);
        } finally {
            lock.unlock();
        }
    }

    /** 清空全局公共属性。 */
    public void clearSuperProperties() {
        lock.lock();
        try {
            superProps.clear();
        } finally {
            lock.unlock();
        }
    }

    /** 上报自定义行为事件。可重试失败留在内存队列等待下次补发。 */
    public void track(String eventName, Map<String, String> properties, TrackOptions options) {
        Map<String, Object> event = buildEvent("BEHAVIOR", eventName, properties, options, null);
        if (event == null) return;
        enqueue(event);
    }

    /** 上报风险事件（进入 uba_risk_events 通道）。 */
    public void trackRisk(String eventName, RiskEvent risk, Map<String, String> properties,
                          TrackOptions options) {
        if (risk == null) return;
        Map<String, Object> event = buildEvent("RISK", eventName, properties, options, risk);
        if (event == null) return;
        enqueue(event);
    }

    /** 页面浏览事件（服务端场景少用，供完整性保留）。 */
    public void trackPageView(String screenName, String referer) {
        long now = System.currentTimeMillis();
        Long dwell = lastEventAtPage == null ? null : now - lastEventAtPage;
        lastEventAtPage = now;
        Map<String, String> props = new LinkedHashMap<>();
        props.put("pageUrl", screenName);
        if (referer != null && !referer.isEmpty()) props.put("referer", referer);
        TrackOptions options = new TrackOptions()
                .eventCategory("navigation")
                .objectType("page")
                .objectId(screenName);
        if (dwell != null && dwell > 0) options.durationMs(dwell);
        track("page_view", props, options);
    }

    private Long lastEventAtPage;

    /** 队列中待上报条数。 */
    public int pendingCount() {
        lock.lock();
        try {
            return queue.size();
        } finally {
            lock.unlock();
        }
    }

    /** 手动冲刷队列：按批量/字节上限分批发送，直到清空或遇到失败。并发调用安全。 */
    public FlushResult flush() {
        lock.lock();
        if (flushing || closed) {
            lock.unlock();
            return new FlushResult(true, 0, 0, false, null);
        }
        flushing = true;
        lock.unlock();

        int sent = 0;
        int dropped = 0;
        boolean ok = true;
        boolean retryable = false;
        String message = null;
        try {
            while (true) {
                List<Map<String, Object>> batch;
                lock.lock();
                try {
                    if (queue.isEmpty()) break;
                    batch = takeBatchLocked();
                } finally {
                    lock.unlock();
                }

                FlushResult r = sendWithRetry(batch);
                sent += r.sent;
                dropped += r.dropped;
                if (!r.success) {
                    ok = false;
                    message = r.message;
                    retryable = r.retryable;
                    // 可重试失败（网络/5xx/429）回到队首等下次补发；确定性 4xx 直接丢弃
                    if (retryable) {
                        lock.lock();
                        try {
                            batch.addAll(queue);
                            queue.clear();
                            queue.addAll(batch);
                        } finally {
                            lock.unlock();
                        }
                    }
                    break;
                }
            }
        } finally {
            lock.lock();
            flushing = false;
            lock.unlock();
        }
        return new FlushResult(ok, sent, dropped, retryable, message);
    }

    /** 关闭：停止定时器并冲刷剩余队列。之后 track 不再入队。 */
    public FlushResult close() {
        if (timer != null) timer.shutdownNow();
        FlushResult result = flush();
        lock.lock();
        try {
            closed = true;
        } finally {
            lock.unlock();
        }
        transport.close();
        return result;
    }

    // ─── AB 实验开关 ──────────────────────────────────────────────────────────

    /** 拉取最新实验开关并缓存。失败时保留旧缓存并抛异常。 */
    public Map<String, String> loadFlags() throws InterruptedException {
        Map<String, Object> body = new LinkedHashMap<>();
        body.put("appId", config.appId());
        body.put("appSecret", config.appSecret());
        body.put("deviceId", deviceId);
        long uid = userId.get();
        if (uid != 0) body.put("userId", uid);

        HttpTransport.HttpResult res;
        try {
            res = transport.postJson(config.flagsUrl(), JsonWriter.write(body), config.timeout());
        } catch (InterruptedException e) {
            throw e;
        } catch (RuntimeException e) {
            return snapshotFlags();
        }
        if (!res.isOk()) {
            throw new IllegalStateException("拉取 flags 失败 HTTP " + res.statusCode);
        }
        // 轻量解析 {"flags": {...}}：只取 flags 对象的字符串键值对
        Map<String, String> parsed = parseFlags(res.body);
        lock.lock();
        try {
            flags.clear();
            flags.putAll(parsed);
        } finally {
            lock.unlock();
        }
        return snapshotFlags();
    }

    /** 读取开关变体；命中时自动上报一次 $feature_flag 曝光事件（每会话每开关一次）。 */
    public String getFlag(String key) {
        String variant;
        lock.lock();
        try {
            variant = flags.get(key);
        } finally {
            lock.unlock();
        }
        if (variant != null) trackFlagExposure(key, variant);
        return variant;
    }

    /** 变体非空且非 control 即视为命中（不触发曝光事件）。 */
    public boolean isFeatureEnabled(String key) {
        lock.lock();
        try {
            String v = flags.get(key);
            return v != null && !v.isEmpty() && !"control".equals(v);
        } finally {
            lock.unlock();
        }
    }

    private Map<String, String> snapshotFlags() {
        lock.lock();
        try {
            return new LinkedHashMap<>(flags);
        } finally {
            lock.unlock();
        }
    }

    // ─── 内部实现 ────────────────────────────────────────────────────────────

    private void flushQuietly() {
        lock.lock();
        boolean empty = queue.isEmpty();
        lock.unlock();
        if (!empty) flush();
    }

    private void enqueue(Map<String, Object> event) {
        boolean autoFlush;
        lock.lock();
        try {
            queue.add(event);
            if (queue.size() > config.maxQueueSize()) {
                queue.subList(0, queue.size() - config.maxQueueSize()).clear();
            }
            autoFlush = queue.size() >= config.batchSize();
        } finally {
            lock.unlock();
        }
        if (autoFlush) {
            Thread flusher = new Thread(this::flush, "uba-auto-flush");
            flusher.setDaemon(true);
            flusher.start();
        }
    }

    private Map<String, Object> buildEvent(String eventType, String eventName,
                                           Map<String, String> properties, TrackOptions options,
                                           RiskEvent risk) {
        if (closed) {
            debug("客户端已关闭，丢弃事件 " + eventName);
            return null;
        }
        String sessionId = currentSessionId();
        if (!sessionInSample(sessionId, config.sampleRate())) return null; // 会话级确定性采样

        TrackOptions o = options;
        long uid = o != null && o.userId != 0 ? o.userId : userId.get();
        String deviceId = o != null && o.deviceId != null && !o.deviceId.isEmpty()
                ? o.deviceId : this.deviceId;
        String session = o != null && o.sessionId != null && !o.sessionId.isEmpty()
                ? o.sessionId : sessionId;
        String platform = o != null && o.platform != null && !o.platform.isEmpty()
                ? o.platform : config.platform();

        // 字段顺序即 JSON 输出顺序（JsonWriter 按插入序）
        Map<String, Object> event = new LinkedHashMap<>();
        event.put("eventType", eventType);
        event.put("eventId", newUuid());
        event.put("eventName", eventName);
        event.put("eventTime", toRFC3339Ms(Instant.now()));
        if (uid != 0) event.put("userId", uid);
        event.put("deviceId", deviceId);
        event.put("sessionId", session);
        event.put("platform", platform);
        putIfPresent(event, "eventCategory", o == null ? null : o.eventCategory);
        Map<String, String> merged = mergedProperties(properties, o);
        if (!merged.isEmpty()) event.put("properties", merged);
        if (o != null && o.metrics != null && !o.metrics.isEmpty()) event.put("metrics", o.metrics);
        putIfPresent(event, "traceId", o == null ? null : o.traceId);
        // 顶层字符串镜像维度；数值标量不落顶层（proto3 无法区分 unset 0，只信 oneof 内的值）
        putIfPresent(event, "eventAction", o == null ? null : o.eventAction);
        putIfPresent(event, "objectType", o == null ? null : o.objectType);
        putIfPresent(event, "objectId", o == null ? null : o.objectId);
        putIfPresent(event, "objectName", o == null ? null : o.objectName);
        putIfPresent(event, "amount", o == null ? null : o.amount);
        putIfPresent(event, "opResult", o == null ? null : o.opResult);
        putIfPresent(event, "errorCode", o == null ? null : o.errorCode);

        if (risk != null) {
            Map<String, Object> riskMap = new LinkedHashMap<>();
            putIfPresent(riskMap, "riskType", risk.riskType);
            putIfPresent(riskMap, "riskLevel", risk.riskLevel);
            if (risk.riskScore != 0) riskMap.put("riskScore", risk.riskScore);
            if (risk.ruleId != 0) riskMap.put("ruleId", risk.ruleId);
            putIfPresent(riskMap, "ruleName", risk.ruleName);
            putIfPresent(riskMap, "ruleContext", risk.ruleContext);
            putIfPresent(riskMap, "relatedEventIds", risk.relatedEventIds);
            putIfPresent(riskMap, "description", risk.description);
            putIfPresent(riskMap, "evidence", risk.evidence);
            putIfPresent(riskMap, "status", risk.status);
            event.put("risk", riskMap);
        } else {
            // behavior oneof 必须带载荷（服务端校验 payload 与 eventType 匹配）
            Map<String, Object> behavior = new LinkedHashMap<>();
            putIfPresent(behavior, "eventAction", o == null ? null : o.eventAction);
            putIfPresent(behavior, "objectType", o == null ? null : o.objectType);
            putIfPresent(behavior, "objectId", o == null ? null : o.objectId);
            putIfPresent(behavior, "objectName", o == null ? null : o.objectName);
            behavior.put("os", osName);
            if (!config.appVersion().isEmpty()) behavior.put("appVersion", config.appVersion());
            if (!config.channel().isEmpty()) behavior.put("channel", config.channel());
            putIfPresent(behavior, "network", o == null ? null : o.network);
            if (o != null && o.durationMs != 0) behavior.put("durationMs", o.durationMs);
            putIfPresent(behavior, "amount", o == null ? null : o.amount);
            if (o != null && o.quantity != 0) behavior.put("quantity", o.quantity);
            if (o != null && o.score != 0) behavior.put("score", o.score);
            if (o != null && o.metrics != null && !o.metrics.isEmpty()) behavior.put("metrics", o.metrics);
            putIfPresent(behavior, "opResult", o == null ? null : o.opResult);
            putIfPresent(behavior, "errorCode", o == null ? null : o.errorCode);
            putIfPresent(behavior, "serverId", o == null ? null : o.serverId);
            if (o != null && o.level != 0) behavior.put("level", o.level);
            event.put("behavior", behavior);
        }
        return event;
    }

    private Map<String, String> mergedProperties(Map<String, String> properties, TrackOptions o) {
        Map<String, String> merged = new LinkedHashMap<>();
        lock.lock();
        try {
            merged.putAll(superProps);
        } finally {
            lock.unlock();
        }
        if (properties != null) merged.putAll(properties);
        if (o != null && o.properties != null) merged.putAll(o.properties);
        return merged;
    }

    private String currentSessionId() {
        Instant now = Instant.now();
        lock.lock();
        try {
            if (!config.sessionTimeout().isZero() && lastEventAt != null
                    && now.toEpochMilli() - lastEventAt.toEpochMilli() > config.sessionTimeout().toMillis()) {
                sessionId = newUuid();
                exposedSessionId = null;
            }
            lastEventAt = now;
            return sessionId;
        } finally {
            lock.unlock();
        }
    }

    /** 取一批：受 batchSize 与 maxBatchBytes 双重限制。调用方需持锁。 */
    private List<Map<String, Object>> takeBatchLocked() {
        List<Map<String, Object>> batch = new ArrayList<>();
        int bytes = 0;
        while (!queue.isEmpty() && batch.size() < config.batchSize()) {
            int encoded = JsonWriter.write(queue.get(0)).length();
            if (!batch.isEmpty() && bytes + encoded > config.maxBatchBytes()) break;
            bytes += encoded;
            batch.add(queue.remove(0));
        }
        return batch;
    }

    private FlushResult sendWithRetry(List<Map<String, Object>> batch) {
        Map<String, Object> envelope = new LinkedHashMap<>();
        envelope.put("appId", config.appId());
        envelope.put("appSecret", config.appSecret());
        Map<String, Object> clientInfo = new LinkedHashMap<>();
        clientInfo.put("userAgent", userAgent);
        envelope.put("clientInfo", clientInfo);
        envelope.put("events", batch);
        String body = JsonWriter.write(envelope);

        for (int attempt = 0; ; attempt++) {
            HttpTransport.HttpResult res;
            try {
                res = transport.postJson(config.reportUrl(), body, config.timeout());
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                return new FlushResult(false, 0, batch.size(), true, e.getMessage());
            }

            if (res.isOk()) {
                int failed = parseFailedCount(res.body);
                if (failed > 0) debug("部分事件被服务端拒绝：HTTP 200 failedCount=" + failed);
                return new FlushResult(true, batch.size() - failed, failed, false, null);
            }

            // 4xx（除 429）是确定性失败：整批丢弃，不重试
            if (res.statusCode >= 400 && res.statusCode < 500 && res.statusCode != 429) {
                debug("HTTP " + res.statusCode + "（不可重试），丢弃 " + batch.size() + " 条");
                return new FlushResult(false, 0, batch.size(), false, "HTTP " + res.statusCode);
            }

            if (attempt >= Math.max(config.maxRetries(), 0)) {
                String reason = res.statusCode == 0
                        ? String.valueOf(res.body) : "HTTP " + res.statusCode;
                debug("重试耗尽（" + reason + "），" + batch.size() + " 条待下次补发");
                return new FlushResult(false, 0, batch.size(), true,
                        reason + " after " + (attempt + 1) + " attempts");
            }

            long delay = config.retryBaseDelay().toMillis() * (1L << attempt);
            debug("HTTP " + res.statusCode + "，" + delay + "ms 后第 " + (attempt + 1) + " 次重试");
            try {
                TimeUnit.MILLISECONDS.sleep(delay);
            } catch (InterruptedException e) {
                Thread.currentThread().interrupt();
                return new FlushResult(false, 0, batch.size(), true, "interrupted");
            }
        }
    }

    private void trackFlagExposure(String key, String variant) {
        String sessionId = currentSessionId();
        boolean first;
        lock.lock();
        try {
            if (!sessionId.equals(exposedSessionId)) {
                exposedFlags.clear();
                exposedSessionId = sessionId;
            }
            first = exposedFlags.add(key);
        } finally {
            lock.unlock();
        }
        if (first) {
            Map<String, String> props = new LinkedHashMap<>();
            props.put("flag_key", key);
            props.put("variant", variant);
            track("$feature_flag", props, new TrackOptions().eventCategory("experiment"));
        }
    }

    private void debug(String message) {
        if (config.debug()) System.out.println("[uba-java] " + message);
    }

    // ─── 静态工具 ────────────────────────────────────────────────────────────

    static String newUuid() {
        return java.util.UUID.randomUUID().toString();
    }

    static String toRFC3339Ms(Instant instant) {
        return RFC3339_MS.format(instant);
    }

    static boolean sessionInSample(String sessionId, double rate) {
        if (rate >= 1) return true;
        if (rate <= 0) return false;
        int h = fnv1a32(sessionId);
        return (h % 1000) / 1000.0 < rate;
    }

    static int fnv1a32(String s) {
        int hash = 0x811c9dc5;
        for (int i = 0; i < s.length(); i++) {
            hash ^= (s.charAt(i) & 0xff);
            hash *= 0x01000193;
        }
        return hash;
    }

    private static String defaultHostname() {
        try {
            String h = InetAddress.getLocalHost().getHostName();
            return h == null || h.isEmpty() ? "unknown-host" : h;
        } catch (Exception e) {
            return "unknown-host";
        }
    }

    private static void putIfPresent(Map<String, Object> map, String key, Object value) {
        if (value == null) return;
        if (value instanceof Map && ((Map<?, ?>) value).isEmpty()) return;
        if (value instanceof List && ((List<?>) value).isEmpty()) return;
        if (value instanceof String && ((String) value).isEmpty()) return;
        map.put(key, value);
    }

    /** 从 {"success":true,"failedCount":2,...} 提取 failedCount（轻量解析，避免引 JSON 库）。 */
    static int parseFailedCount(String body) {
        if (body == null) return 0;
        int idx = body.indexOf("\"failedCount\"");
        if (idx < 0) return 0;
        int colon = body.indexOf(':', idx);
        if (colon < 0) return 0;
        StringBuilder digits = new StringBuilder();
        for (int i = colon + 1; i < body.length(); i++) {
            char c = body.charAt(i);
            if (Character.isDigit(c)) digits.append(c);
            else if (digits.length() > 0) break;
            else if (!Character.isWhitespace(c)) break;
        }
        if (digits.length() == 0) return 0;
        try {
            return Integer.parseInt(digits.toString());
        } catch (NumberFormatException e) {
            return 0;
        }
    }

    /** 从 {"flags": {"k":"v"}} 轻量提取键值对。 */
    static Map<String, String> parseFlags(String body) {
        Map<String, String> out = new LinkedHashMap<>();
        if (body == null) return out;
        int flagsIdx = body.indexOf("\"flags\"");
        if (flagsIdx < 0) return out;
        int objStart = body.indexOf('{', flagsIdx);
        if (objStart < 0) return out;
        int depth = 0;
        int end = -1;
        for (int i = objStart; i < body.length(); i++) {
            char c = body.charAt(i);
            if (c == '{') depth++;
            else if (c == '}') {
                depth--;
                if (depth == 0) {
                    end = i;
                    break;
                }
            }
        }
        if (end < 0) return out;
        String inner = body.substring(objStart + 1, end);
        // 逐个 "k":"v" 对解析（值内不含转义引号的常规开关场景）
        int i = 0;
        while (i < inner.length()) {
            int keyStart = inner.indexOf('"', i);
            if (keyStart < 0) break;
            int keyEnd = inner.indexOf('"', keyStart + 1);
            if (keyEnd < 0) break;
            int colon = inner.indexOf(':', keyEnd + 1);
            if (colon < 0) break;
            int valStart = inner.indexOf('"', colon + 1);
            if (valStart < 0) break;
            int valEnd = inner.indexOf('"', valStart + 1);
            if (valEnd < 0) break;
            out.put(inner.substring(keyStart + 1, keyEnd), inner.substring(valStart + 1, valEnd));
            i = valEnd + 1;
            while (i < inner.length() && (inner.charAt(i) == ',' || inner.charAt(i) == ' ')) i++;
        }
        return out;
    }
}
