package com.gowind.uba;

import java.util.LinkedHashMap;
import java.util.Map;

/** 单次上报维度，字段与其他端 SDK 的 TrackOptions 一一对应。 */
public final class TrackOptions {

    String eventCategory;
    long userId;
    String deviceId;
    String sessionId;
    String platform;
    String eventAction;
    String objectType;
    String objectId;
    String objectName;
    long durationMs;
    String amount;
    long quantity;
    int score;
    Map<String, Double> metrics;
    String network;
    String opResult;
    String errorCode;
    Map<String, String> properties;
    String serverId;
    int level;
    String traceId;

    public TrackOptions eventCategory(String v) { this.eventCategory = v; return this; }
    public TrackOptions userId(long v) { this.userId = v; return this; }
    public TrackOptions deviceId(String v) { this.deviceId = v; return this; }
    public TrackOptions sessionId(String v) { this.sessionId = v; return this; }
    public TrackOptions platform(String v) { this.platform = v; return this; }
    public TrackOptions eventAction(String v) { this.eventAction = v; return this; }
    public TrackOptions objectType(String v) { this.objectType = v; return this; }
    public TrackOptions objectId(String v) { this.objectId = v; return this; }
    public TrackOptions objectName(String v) { this.objectName = v; return this; }
    public TrackOptions durationMs(long v) { this.durationMs = v; return this; }
    public TrackOptions amount(String v) { this.amount = v; return this; }
    public TrackOptions quantity(long v) { this.quantity = v; return this; }
    public TrackOptions score(int v) { this.score = v; return this; }
    public TrackOptions metrics(Map<String, Double> v) { this.metrics = v; return this; }
    public TrackOptions network(String v) { this.network = v; return this; }
    public TrackOptions opResult(String v) { this.opResult = v; return this; }
    public TrackOptions errorCode(String v) { this.errorCode = v; return this; }
    public TrackOptions properties(Map<String, String> v) { this.properties = v; return this; }
    public TrackOptions serverId(String v) { this.serverId = v; return this; }
    public TrackOptions level(int v) { this.level = v; return this; }
    public TrackOptions traceId(String v) { this.traceId = v; return this; }

    /** 业务属性便捷构造：{@code P.of("k1","v1","k2","v2")}。 */
    public static Map<String, String> p(String... kv) {
        if (kv.length % 2 != 0) throw new IllegalArgumentException("属性必须是 key-value 成对");
        Map<String, String> m = new LinkedHashMap<>();
        for (int i = 0; i < kv.length; i += 2) m.put(kv[i], kv[i + 1]);
        return m;
    }
}
