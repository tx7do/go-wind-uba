"""契约金样：wire dict 键集合/位置/类型必须与后端 proto 契约完全一致。"""

import json
import unittest

from uba.config import RiskEvent, TrackOptions
from uba.json_wire import build_behavior, build_event, envelope, parse_failed_count, parse_flags


def build_full_event():
    opts = TrackOptions(
        event_category="order",
        event_action="pay",
        object_type="order",
        object_id="ORD-001",
        object_name="订单",
        duration_ms=3500,
        amount="99.90",
        quantity=2,
        score=88,
        metrics={"amount_num": 99.9},
        network="WiFi",
        op_result="success",
        server_id="s1",
        level=42,
        trace_id="trace-1",
        user_id=1001,
    )
    behavior = build_behavior(opts, os_name="Linux", app_version="1.0.0", channel="official")
    return build_event(
        "BEHAVIOR", "evt-001", "order_paid", "2026-03-27T12:00:00.123Z",
        opts, {"orderId": "ORD-001", "env": "prod"},
        user_id=2002, device_id="host-1", session_id="sess-1", platform="server",
        behavior=behavior, risk=None,
    )


class GoldenPayloadTest(unittest.TestCase):
    def test_full_behavior_event(self):
        m = build_full_event()

        # 必填四件套
        self.assertEqual(m["eventType"], "BEHAVIOR")
        self.assertEqual(m["eventName"], "order_paid")
        self.assertEqual(m["eventTime"], "2026-03-27T12:00:00.123Z")
        # 数值标量绝不出现在顶层（proto3 顶层无法区分 unset 0，服务端只信 oneof）
        for key in ("tenantId", "serverTime", "ip",
                    "durationMs", "quantity", "score", "level", "sessionSeq"):
            self.assertNotIn(key, m, key)
        # 身份与环境（TrackOptions.user_id 覆盖 identify 传入）
        self.assertEqual(m["userId"], 1001)
        self.assertEqual(m["deviceId"], "host-1")
        self.assertEqual(m["sessionId"], "sess-1")
        self.assertEqual(m["platform"], "server")
        # 属性与字符串镜像维度
        self.assertEqual(m["properties"], {"orderId": "ORD-001", "env": "prod"})
        self.assertEqual(m["metrics"], {"amount_num": 99.9})
        self.assertEqual(m["eventAction"], "pay")
        self.assertEqual(m["objectType"], "order")
        self.assertEqual(m["objectId"], "ORD-001")
        self.assertEqual(m["amount"], "99.90")
        self.assertEqual(m["opResult"], "success")
        # 顶层键集合精确
        expected = {
            "eventType", "eventId", "eventName", "eventTime", "userId", "deviceId",
            "sessionId", "platform", "eventCategory", "properties", "metrics",
            "traceId", "eventAction", "objectType", "objectId", "objectName",
            "amount", "opResult", "behavior",
        }
        self.assertEqual(set(m.keys()), expected)

        # behavior oneof：数值标量在这里才被信任
        b = m["behavior"]
        self.assertEqual(b["durationMs"], 3500)
        self.assertEqual(b["quantity"], 2)
        self.assertEqual(b["score"], 88)
        self.assertEqual(b["level"], 42)
        self.assertEqual(b["serverId"], "s1")
        self.assertEqual(b["os"], "Linux")
        self.assertEqual(b["appVersion"], "1.0.0")
        self.assertEqual(b["channel"], "official")
        self.assertEqual(b["network"], "WiFi")

    def test_envelope_serialization(self):
        m = build_full_event()
        body = envelope("app_001", "secret_001", "uba-python/1.0.0 (linux)", [m])
        parsed = json.loads(body)
        self.assertEqual(parsed["appId"], "app_001")
        self.assertEqual(parsed["appSecret"], "secret_001")
        self.assertEqual(parsed["clientInfo"]["userAgent"], "uba-python/1.0.0 (linux)")
        self.assertEqual(len(parsed["events"]), 1)
        # userId 是数字不是字符串
        self.assertEqual(parsed["events"][0]["userId"], 1001)

    def test_risk_event(self):
        risk = RiskEvent(
            risk_type="login_anomaly",
            risk_level="HIGH",
            risk_score=88.5,
            rule_id=101,
            rule_name="1分钟内登录失败超过5次",
            rule_context={"threshold": 5, "window_sec": 60},
            related_event_ids=["e1", "e2"],
            description="失败8次",
            evidence={"ip": "1.2.3.4"},
            status="PENDING",
        )
        m = build_event(
            "RISK", "risk-001", "login_failed", "2026-03-27T12:00:00.123Z",
            None, {"try_count": "8"},
            user_id=1001, device_id="host-1", session_id="sess-1", platform="server",
            behavior=None, risk=risk,
        )
        self.assertEqual(m["eventType"], "RISK")
        self.assertNotIn("behavior", m)
        self.assertEqual(m["userId"], 1001)
        risk_wire = m["risk"]
        self.assertEqual(risk_wire["riskType"], "login_anomaly")
        self.assertEqual(risk_wire["riskLevel"], "HIGH")
        self.assertEqual(risk_wire["riskScore"], 88.5)
        self.assertEqual(risk_wire["ruleId"], 101)
        self.assertEqual(risk_wire["ruleContext"], {"threshold": 5, "window_sec": 60})
        self.assertEqual(risk_wire["relatedEventIds"], ["e1", "e2"])
        self.assertEqual(risk_wire["status"], "PENDING")
        # 服务端补全字段不发送
        self.assertNotIn("occurTime", risk_wire)
        self.assertNotIn("reportTime", risk_wire)

    def test_minimal_event(self):
        m = build_event(
            "BEHAVIOR", "e1", "heartbeat", "2026-03-27T12:00:00.123Z",
            None, None,
            user_id=None, device_id="host-1", session_id="sess-1", platform="server",
            behavior=build_behavior(None, "Linux", "", ""),
            risk=None,
        )
        self.assertNotIn("userId", m)
        self.assertNotIn("properties", m)
        self.assertNotIn("metrics", m)
        self.assertEqual(m["behavior"], {"os": "Linux"})

    def test_parse_helpers(self):
        self.assertEqual(parse_failed_count(
            '{"success": true, "totalCount": 3, "successCount": 1, "failedCount": 2}'), 2)
        self.assertEqual(parse_failed_count('{"success": true}'), 0)
        self.assertEqual(parse_failed_count("not json"), 0)
        flags = parse_flags('{"success":true,"flags":{"exp":"red","ctl":"control"}}')
        self.assertEqual(flags, {"exp": "red", "ctl": "control"})
        self.assertEqual(parse_flags("not json"), {})


if __name__ == "__main__":
    unittest.main()
