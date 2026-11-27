"""客户端行为测试：http.server 桩服务（可编程状态码队列）+ FakeTransport。"""

import json
import threading
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

from uba.client import UbaClient
from uba.config import FlushResult, RiskEvent, TrackOptions, UbaConfig
from uba.transport import HttpResult, HttpTransport
from uba.utils import uuid_v4

RESP_OK = '{"success": true, "totalCount": 1, "successCount": 1, "failedCount": 0}'


class StubServer:
    """可编程状态码队列的本地桩服务。"""

    def __init__(self):
        self.bodies = []
        self.status_queue = []
        self.lock = threading.Lock()
        self._server = None
        self._port = None

    def start(self):
        outer = self

        class Handler(BaseHTTPRequestHandler):
            def do_POST(self):
                length = int(self.headers.get("Content-Length", 0))
                body = self.rfile.read(length).decode("utf-8")
                with outer.lock:
                    outer.bodies.append(body)
                    status = outer.status_queue.pop(0) if outer.status_queue else 200
                resp = RESP_OK.encode("utf-8") if status == 200 else b"error"
                self.send_response(status)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(resp)))
                self.end_headers()
                self.wfile.write(resp)

            def log_message(self, *args):  # 静默
                pass

        self._server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self._port = self._server.server_address[1]
        thread = threading.Thread(target=self._server.serve_forever, daemon=True)
        thread.start()

    def stop(self):
        if self._server:
            self._server.shutdown()
            self._server.server_close()

    @property
    def url(self):
        return f"http://127.0.0.1:{self._port}"

    def last_body(self):
        return self.bodies[-1]


def make_config(url, **overrides):
    params = dict(
        app_id="app_001",
        app_secret="secret_001",
        endpoint=url,
        batch_size=100,
        flush_interval_ms=0,   # 测试禁用定时器
        max_retries=1,
        retry_base_delay_ms=1,
    )
    params.update(overrides)
    return UbaConfig(**params)


class FakeTransport(HttpTransport):
    """编程式假传输（不经网络）。"""

    def __init__(self, statuses=None):
        self.requests = []
        self.statuses = list(statuses or [])

    def send(self, url, body, timeout_ms):
        self.requests.append((url, json.loads(body)))
        if self.statuses:
            status = self.statuses.pop(0)
        else:
            status = 200
        return HttpResult(status, RESP_OK if status == 200 else "error")

    @property
    def last_body(self):
        return self.requests[-1][1]


class ClientGoldenTest(unittest.TestCase):
    def setUp(self):
        self.server = StubServer()
        self.server.start()

    def tearDown(self):
        self.server.stop()

    def test_golden_and_envelope(self):
        client = UbaClient(make_config(self.server.url))
        client.identify(1001)
        client.set_super_properties({"env": "prod"})
        client.track(
            "order_paid", {"orderId": "ORD-001"},
            TrackOptions(
                event_category="order", event_action="pay",
                object_type="order", object_id="ORD-001", object_name="订单",
                duration_ms=3500, amount="99.90", quantity=2, score=88,
                metrics={"amount_num": 99.9}, network="WiFi",
                op_result="success", server_id="s1", level=42, trace_id="trace-1",
            ),
        )
        r = client.flush()
        self.assertTrue(r.success)
        self.assertEqual(r.sent, 1)
        client.close()

        body = json.loads(self.server.last_body())
        self.assertEqual(body["appId"], "app_001")
        self.assertEqual(body["appSecret"], "secret_001")
        self.assertTrue(body["clientInfo"]["userAgent"].startswith("uba-python/"))
        evt = body["events"][0]
        self.assertEqual(evt["eventType"], "BEHAVIOR")
        self.assertEqual(evt["userId"], 1001)
        self.assertEqual(evt["properties"]["env"], "prod")
        for key in ("tenantId", "serverTime", "ip",
                    "durationMs", "quantity", "score", "level"):
            self.assertNotIn(key, evt)
        self.assertEqual(evt["behavior"]["durationMs"], 3500)
        self.assertEqual(evt["behavior"]["level"], 42)
        self.assertEqual(evt["behavior"]["serverId"], "s1")

    def test_track_risk(self):
        client = UbaClient(make_config(self.server.url))
        client.track_risk(
            "login_failed",
            RiskEvent(risk_type="login_anomaly", risk_level="HIGH",
                      risk_score=88.5, rule_id=101, status="PENDING"),
            {"try_count": "8"},
        )
        client.flush()
        client.close()
        evt = json.loads(self.server.last_body())["events"][0]
        self.assertEqual(evt["eventType"], "RISK")
        self.assertNotIn("behavior", evt)
        self.assertEqual(evt["risk"]["riskScore"], 88.5)

    def test_no_retry_on_401(self):
        transport = FakeTransport(statuses=[401])
        client = UbaClient(make_config("http://stub"), transport=transport)
        client.track("t")
        r = client.flush()
        self.assertFalse(r.success)
        self.assertFalse(r.retryable)
        self.assertEqual(r.dropped, 1)
        self.assertEqual(len(transport.requests), 1)
        self.assertEqual(client.pending_count(), 0)
        client.close()

    def test_retry_on_5xx_until_success(self):
        transport = FakeTransport(statuses=[500, 503])
        client = UbaClient(make_config("http://stub", max_retries=2), transport=transport)
        client.track("t")
        r = client.flush()
        self.assertTrue(r.success)
        self.assertEqual(len(transport.requests), 3)
        client.close()

    def test_retry_exhausted_stays_queued(self):
        transport = FakeTransport(statuses=[500, 500])  # flush 重试耗尽
        client = UbaClient(make_config("http://stub"), transport=transport)
        client.track("offline-event", {"k": "v"})
        r = client.flush()
        self.assertFalse(r.success)
        self.assertTrue(r.retryable)
        self.assertEqual(client.pending_count(), 1)  # 回队等待补发
        # 服务恢复（statuses 耗尽 → 200）后再 flush 补发成功
        r2 = client.flush()
        self.assertTrue(r2.success)
        self.assertEqual(client.pending_count(), 0)
        client.close()
        sent = transport.last_body["events"][0]
        self.assertEqual(sent["eventName"], "offline-event")
        self.assertEqual(sent["properties"]["k"], "v")

    def test_partial_failure_on_200(self):
        class PartialTransport(FakeTransport):
            def send(self, url, body, timeout_ms):
                self.requests.append((url, json.loads(body)))
                return HttpResult(200, '{"success": true, "totalCount": 3, '
                                       '"successCount": 1, "failedCount": 2}')

        client = UbaClient(make_config("http://stub"), transport=PartialTransport())
        for name in ("a", "b", "c"):
            client.track(name)
        r = client.flush()
        self.assertTrue(r.success)  # HTTP 200 部分失败仍算传输成功
        self.assertEqual(r.sent, 1)
        self.assertEqual(r.dropped, 2)
        self.assertEqual(client.pending_count(), 0)
        client.close()

    def test_batch_size_auto_flush(self):
        transport = FakeTransport()
        client = UbaClient(make_config("http://stub", batch_size=2), transport=transport)
        client.track("a")
        client.track("b")
        deadline_result = []
        import time
        for _ in range(100):
            if transport.requests:
                break
            time.sleep(0.01)
        deadline_result.append(len(transport.requests))
        self.assertEqual(deadline_result[0], 1)
        client.close()

    def test_concurrent_track(self):
        transport = FakeTransport()
        client = UbaClient(make_config("http://stub"), transport=transport)
        threads = []
        for _ in range(20):
            def worker():
                for _ in range(50):
                    client.track("concurrent")
            t = threading.Thread(target=worker)
            threads.append(t)
            t.start()
        for t in threads:
            t.join()
        # 等自动 flush 清空队列（防重入下多次 flush 兜底），统计服务端收到总数
        import time
        for _ in range(300):
            if client.pending_count() == 0 and len(transport.requests) >= 10:
                break
            client.flush()
            time.sleep(0.01)
        client.flush()
        total = sum(len(body["events"]) for _, body in transport.requests)
        self.assertEqual(total, 1000)
        client.close()

    def test_sampling_off_drops_all(self):
        transport = FakeTransport()
        client = UbaClient(make_config("http://stub", sample_rate=0.0), transport=transport)
        self.assertFalse(client.is_collecting())
        for _ in range(10):
            client.track("t")
        self.assertEqual(client.pending_count(), 0)
        client.close()

    def test_track_options_override_identity(self):
        transport = FakeTransport()
        client = UbaClient(make_config("http://stub"), transport=transport)
        client.identify(1)
        client.track("t", None, TrackOptions(user_id=999, device_id="custom",
                                             session_id="custom-sess", platform="game"))
        client.flush()
        client.close()
        evt = transport.last_body["events"][0]
        self.assertEqual(evt["userId"], 999)
        self.assertEqual(evt["deviceId"], "custom")
        self.assertEqual(evt["sessionId"], "custom-sess")
        self.assertEqual(evt["platform"], "game")

    def test_closed_client_rejects(self):
        transport = FakeTransport()
        client = UbaClient(make_config("http://stub"), transport=transport)
        client.track("last")
        client.close()
        before = len(transport.requests)
        client.track("after-close")
        self.assertEqual(client.pending_count(), 0)
        client.flush()
        self.assertEqual(len(transport.requests), before)

    def test_config_validation(self):
        cases = [
            dict(app_id="", app_secret="s", endpoint="e"),
            dict(app_id="a", app_secret="", endpoint="e"),
            dict(app_id="a", app_secret="s", endpoint=""),
            dict(app_id="a", app_secret="s", endpoint="e", timeout_ms=10000),
            dict(app_id="a", app_secret="s", endpoint="e", max_batch_bytes=1024 * 1024),
            dict(app_id="a", app_secret="s", endpoint="e", batch_size=0),
            dict(app_id="a", app_secret="s", endpoint="e", sample_rate=1.5),
        ]
        for kwargs in cases:
            with self.assertRaises(ValueError):
                UbaConfig(**kwargs)

    def test_session_rotation_on_timeout(self):
        client = UbaClient(make_config(self.server.url, session_timeout_ms=30))
        client.track("a")
        client.flush()
        sess1 = json.loads(self.server.last_body())["events"][0]["sessionId"]
        import time
        time.sleep(0.04)
        client.track("b")
        client.flush()
        client.close()
        sess2 = json.loads(self.server.last_body())["events"][0]["sessionId"]
        self.assertNotEqual(sess1, sess2, f"超时后应轮换会话: {sess1} vs {sess2}")

    def test_short_background_keeps_session(self):
        client = UbaClient(make_config(self.server.url, session_timeout_ms=60000))
        client.track("a")
        client.handle_background()
        client.handle_foreground()
        client.track("b")
        client.flush()
        client.close()
        sessions = [json.loads(b)["events"][0]["sessionId"] for b in self.server.bodies]
        self.assertEqual(len(set(sessions)), 1, "短后台不应轮换 sessionId")

    def test_page_view_dwell_and_referer(self):
        client = UbaClient(make_config(self.server.url))
        client.track_page_view("HomeService")
        import time
        time.sleep(0.005)
        client.track_page_view("DetailService", title="详情", referer="HomeService")
        client.flush()
        client.close()
        evt = json.loads(self.server.last_body())["events"][-1]
        self.assertEqual(evt["eventName"], "page_view")
        self.assertEqual(evt["properties"]["referer"], "HomeService")
        self.assertGreater(evt["behavior"]["durationMs"], 0)


if __name__ == "__main__":
    unittest.main()
