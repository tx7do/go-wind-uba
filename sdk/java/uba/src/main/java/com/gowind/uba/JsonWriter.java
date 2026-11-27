package com.gowind.uba;

import java.util.List;
import java.util.Map;

/**
 * 手写零依赖 JSON 序列化（对齐 C# Uba.Core Json.cs 的约定）：
 * 按 LinkedHashMap 插入顺序输出，跳过 null 值与空集合，camelCase 由调用方键名保证。
 * 服务端为 protojson 编码，数字/布尔/字符串/对象/数组足够覆盖上报契约。
 */
final class JsonWriter {

    private final StringBuilder sb = new StringBuilder(256);

    static String write(Map<String, Object> map) {
        JsonWriter w = new JsonWriter();
        w.writeMap(map);
        return w.sb.toString();
    }

    private void writeMap(Map<String, Object> map) {
        sb.append('{');
        boolean first = true;
        for (Map.Entry<String, Object> e : map.entrySet()) {
            if (skip(e.getValue())) continue;
            if (!first) sb.append(',');
            first = false;
            writeString(e.getKey());
            sb.append(':');
            writeValue(e.getValue());
        }
        sb.append('}');
    }

    private void writeArray(List<Object> list) {
        sb.append('[');
        boolean first = true;
        for (Object v : list) {
            if (skip(v)) continue;
            if (!first) sb.append(',');
            first = false;
            writeValue(v);
        }
        sb.append(']');
    }

    private void writeValue(Object v) {
        if (v == null) {
            sb.append("null");
        } else if (v instanceof Map<?, ?>) {
            @SuppressWarnings("unchecked")
            Map<String, Object> m = (Map<String, Object>) v;
            writeMap(m);
        } else if (v instanceof List<?>) {
            writeArray((List<Object>) v);
        } else if (v instanceof String) {
            writeString((String) v);
        } else if (v instanceof Double || v instanceof Float) {
            double d = ((Number) v).doubleValue();
            if (d == Math.rint(d) && !Double.isInfinite(d) && Math.abs(d) < 1e15) {
                sb.append((long) d);
            } else {
                sb.append(d);
            }
        } else if (v instanceof Number) {
            sb.append(v);
        } else if (v instanceof Boolean) {
            sb.append(v);
        } else {
            writeString(String.valueOf(v));
        }
    }

    private static boolean skip(Object v) {
        if (v == null) return true;
        if (v instanceof Map && ((Map<?, ?>) v).isEmpty()) return true;
        return v instanceof List && ((List<?>) v).isEmpty();
    }

    private void writeString(String s) {
        sb.append('"');
        for (int i = 0; i < s.length(); i++) {
            char c = s.charAt(i);
            if (c == '"') {
                sb.append("\\\"");
            } else if (c == '\\') {
                sb.append("\\\\");
            } else if (c == '\n') {
                sb.append("\\n");
            } else if (c == '\r') {
                sb.append("\\r");
            } else if (c == '\t') {
                sb.append("\\t");
            } else if (c == '\b') {
                sb.append("\\b");
            } else if (c == '\f') {
                sb.append("\\f");
            } else if (c < 0x20) {
                sb.append(String.format("\\u%04x", (int) c));
            } else {
                sb.append(c);
            }
        }
        sb.append('"');
    }
}
