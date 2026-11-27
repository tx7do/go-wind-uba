package com.gowind.uba;

import java.time.Duration;

/**
 * SDK 配置。默认值与其他端 SDK 对齐：批量 20 条 / 5s 定时 / 3 次重试 / 1s 退避基数 /
 * 8s 超时（必须小于服务端 10s）。
 */
public final class UbaConfig {

    private String appId = "";
    private String appSecret = "";
    private String endpoint = "";
    private String path = "/uba/v1/report";
    private String flagsPath = "/uba/v1/flags";
    private int batchSize = 20;
    private Duration flushInterval = Duration.ofSeconds(5);
    private int maxRetries = 3;
    private Duration timeout = Duration.ofSeconds(8);
    private Duration retryBaseDelay = Duration.ofSeconds(1);
    private boolean debug = false;
    /** 服务端实例标识（deviceId 语义），为空自动取 hostname。 */
    private String deviceId = "";
    /** 上报平台标识，默认 "server"。 */
    private String platform = "server";
    private String appVersion = "";
    private String channel = "";
    /** 会话超时：常驻进程默认不轮换（Duration.ZERO）。 */
    private Duration sessionTimeout = Duration.ZERO;
    /** 采样率 0~1，按 sessionId 确定性采样，默认 1。 */
    private double sampleRate = 1.0;
    /** 单批请求体字节上限；服务端对未鉴权大包直接 401，必须压在 1MiB 内。 */
    private int maxBatchBytes = 900 * 1024;
    /** 内存队列上限（防持续故障下无界增长）。 */
    private int maxQueueSize = 10000;

    public UbaConfig appId(String v) { this.appId = v; return this; }
    public UbaConfig appSecret(String v) { this.appSecret = v; return this; }
    public UbaConfig endpoint(String v) { this.endpoint = v; return this; }
    public UbaConfig path(String v) { this.path = v; return this; }
    public UbaConfig flagsPath(String v) { this.flagsPath = v; return this; }
    public UbaConfig batchSize(int v) { this.batchSize = v; return this; }
    public UbaConfig flushInterval(Duration v) { this.flushInterval = v; return this; }
    public UbaConfig maxRetries(int v) { this.maxRetries = v; return this; }
    public UbaConfig timeout(Duration v) { this.timeout = v; return this; }
    public UbaConfig retryBaseDelay(Duration v) { this.retryBaseDelay = v; return this; }
    public UbaConfig debug(boolean v) { this.debug = v; return this; }
    public UbaConfig deviceId(String v) { this.deviceId = v; return this; }
    public UbaConfig platform(String v) { this.platform = v; return this; }
    public UbaConfig appVersion(String v) { this.appVersion = v; return this; }
    public UbaConfig channel(String v) { this.channel = v; return this; }
    public UbaConfig sessionTimeout(Duration v) { this.sessionTimeout = v; return this; }
    public UbaConfig sampleRate(double v) { this.sampleRate = v; return this; }
    public UbaConfig maxBatchBytes(int v) { this.maxBatchBytes = v; return this; }
    public UbaConfig maxQueueSize(int v) { this.maxQueueSize = v; return this; }

    String appId() { return appId; }
    String appSecret() { return appSecret; }
    String endpoint() { return endpoint; }
    String path() { return path; }
    String flagsPath() { return flagsPath; }
    int batchSize() { return batchSize; }
    Duration flushInterval() { return flushInterval; }
    int maxRetries() { return maxRetries; }
    Duration timeout() { return timeout; }
    Duration retryBaseDelay() { return retryBaseDelay; }
    boolean debug() { return debug; }
    String deviceId() { return deviceId; }
    String platform() { return platform; }
    String appVersion() { return appVersion; }
    String channel() { return channel; }
    Duration sessionTimeout() { return sessionTimeout; }
    double sampleRate() { return sampleRate; }
    int maxBatchBytes() { return maxBatchBytes; }
    int maxQueueSize() { return maxQueueSize; }

    String reportUrl() { return trimSlash(endpoint) + path; }
    String flagsUrl() { return trimSlash(endpoint) + flagsPath; }

    private static String trimSlash(String s) {
        return s.endsWith("/") ? s.substring(0, s.length() - 1) : s;
    }

    /** 校验必填与契约约束，非法抛 IllegalArgumentException。 */
    void validate() {
        requireNonEmpty(appId, "appId 不能为空");
        requireNonEmpty(appSecret, "appSecret 不能为空");
        requireNonEmpty(endpoint, "endpoint 不能为空");
        if (timeout.isZero() || timeout.isNegative() || timeout.compareTo(Duration.ofSeconds(10)) >= 0) {
            throw new IllegalArgumentException("timeout 必须 > 0 且 < 10s（服务端超时 10s）");
        }
        if (maxBatchBytes <= 0 || maxBatchBytes >= 1024 * 1024) {
            throw new IllegalArgumentException("maxBatchBytes 必须压在 1MiB 以内");
        }
        if (batchSize <= 0) throw new IllegalArgumentException("batchSize 必须 > 0");
        if (sampleRate < 0 || sampleRate > 1) throw new IllegalArgumentException("sampleRate 必须在 0~1");
        // 0 = 未设置 → 默认 3；负数 = 显式禁用重试
        if (maxRetries == 0) maxRetries = 3;
        else if (maxRetries < 0) maxRetries = 0;
    }

    private static void requireNonEmpty(String v, String message) {
        if (v == null || v.isEmpty()) throw new IllegalArgumentException(message);
    }
}
