package com.gowind.uba;

import org.junit.jupiter.api.Test;

import java.time.Instant;
import java.util.LinkedHashMap;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

/** JSON 序列化与工具函数（对齐其他端 SDK 的 utils 测试）。 */
class JsonAndUtilsTest {

    @Test
    void jsonSkipsNullAndEmpty() {
        Map<String, Object> m = new LinkedHashMap<>();
        m.put("a", null);
        m.put("b", new LinkedHashMap<>());
        m.put("c", new java.util.ArrayList<>());
        m.put("d", "x");
        m.put("e", 0);
        m.put("f", 88.5);
        m.put("g", true);
        assertEquals("{\"d\":\"x\",\"e\":0,\"f\":88.5,\"g\":true}", JsonWriter.write(m));
    }

    @Test
    void jsonEscapesSpecialChars() {
        Map<String, Object> m = new LinkedHashMap<>();
        m.put("v", "引号\"反斜杠\\换行\n制表\t");
        assertEquals("{\"v\":\"引号\\\"反斜杠\\\\换行\\n制表\\t\"}", JsonWriter.write(m));
    }

    @Test
    void jsonNestedMapsAndArrays() {
        Map<String, Object> inner = new LinkedHashMap<>();
        inner.put("threshold", 5);
        Map<String, Object> m = new LinkedHashMap<>();
        m.put("ruleContext", inner);
        m.put("ids", java.util.List.of("e1", "e2"));
        assertEquals("{\"ruleContext\":{\"threshold\":5},\"ids\":[\"e1\",\"e2\"]}", JsonWriter.write(m));
    }

    @Test
    void rfc3339MillisUtc() {
        Instant t = Instant.parse("2026-03-27T12:00:00.123Z");
        assertEquals("2026-03-27T12:00:00.123Z", UbaClient.toRFC3339Ms(t));
        t = Instant.parse("2026-01-02T03:04:05.006Z");
        assertEquals("2026-01-02T03:04:05.006Z", UbaClient.toRFC3339Ms(t));
    }

    @Test
    void uuidFormat() {
        java.util.regex.Pattern p = java.util.regex.Pattern.compile(
                "^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$");
        for (int i = 0; i < 100; i++) {
            assertTrue(p.matcher(UbaClient.newUuid()).matches(), UbaClient.newUuid());
        }
    }

    @Test
    void samplingDeterministic() {
        assertTrue(UbaClient.sessionInSample("s", 1.0));
        assertFalse(UbaClient.sessionInSample("s", 0.0));
        boolean r = UbaClient.sessionInSample("fixed-session", 0.5);
        for (int i = 0; i < 10; i++) {
            assertEquals(r, UbaClient.sessionInSample("fixed-session", 0.5), "同会话采样应恒定");
        }
    }

    @Test
    void parseFailedCount() {
        assertEquals(0, UbaClient.parseFailedCount("{\"success\":true}"));
        assertEquals(2, UbaClient.parseFailedCount(
                "{\"success\": true, \"totalCount\": 3, \"successCount\": 1, \"failedCount\": 2}"));
        assertEquals(0, UbaClient.parseFailedCount("not json"));
        assertEquals(0, UbaClient.parseFailedCount(null));
    }

    @Test
    void parseFlags() {
        Map<String, String> flags = UbaClient.parseFlags(
                "{\"success\":true,\"flags\":{\"exp\":\"red\",\"ctl\":\"control\"}}");
        assertEquals("red", flags.get("exp"));
        assertEquals("control", flags.get("ctl"));
        assertTrue(UbaClient.parseFlags("{\"success\":true}").isEmpty());
        assertTrue(UbaClient.parseFlags(null).isEmpty());
    }

    @Test
    void configValidation() {
        assertThrows(() -> new UbaConfig().appId("").appSecret("s").endpoint("e").validate());
        assertThrows(() -> new UbaConfig().appId("a").appSecret("").endpoint("e").validate());
        assertThrows(() -> new UbaConfig().appId("a").appSecret("s").endpoint("").validate());
        assertThrows(() -> new UbaConfig().appId("a").appSecret("s").endpoint("e")
                .timeout(java.time.Duration.ofSeconds(10)).validate());
        assertThrows(() -> new UbaConfig().appId("a").appSecret("s").endpoint("e")
                .maxBatchBytes(1024 * 1024).validate());

        // 0 = 未设置 → 默认 3；负数 = 禁用重试
        UbaConfig zero = new UbaConfig().appId("a").appSecret("s").endpoint("http://x").maxRetries(0);
        zero.validate();
        assertEquals(3, zero.maxRetries());
        UbaConfig disabled = new UbaConfig().appId("a").appSecret("s").endpoint("http://x").maxRetries(-1);
        disabled.validate();
        assertEquals(0, disabled.maxRetries());
    }

    @Test
    void configUrlsTrimTrailingSlash() {
        UbaConfig cfg = new UbaConfig().appId("a").appSecret("s").endpoint("http://x/");
        assertEquals("http://x/uba/v1/report", cfg.reportUrl());
        assertEquals("http://x/uba/v1/flags", cfg.flagsUrl());
    }

    private static void assertThrows(Runnable r) {
        try {
            r.run();
            throw new AssertionError("应抛 IllegalArgumentException");
        } catch (IllegalArgumentException expected) {
            // 预期
        }
    }
}
