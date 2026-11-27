"""GoWind UBA Python 服务端埋点客户端。线程安全（内部锁 + 守护定时线程）。

用法::

    from uba import UbaClient, UbaConfig, TrackOptions, RiskEvent

    client = UbaClient(UbaConfig(
        app_id="your_app_id",
        app_secret="your_app_secret",
        endpoint="http://localhost:5700",
    ))
    client.identify(1001)
    client.track("order_paid", {"orderId": "ORD-001"},
                 TrackOptions(object_type="order", amount="99.90"))
    client.track_risk("login_failed",
                      RiskEvent(risk_type="login_anomaly", risk_level="HIGH", risk_score=88.5))
    client.close()

契约铁律（与其他端 SDK 一致）：camelCase 对齐 proto json_name；tenantId/serverTime/ip
服务端补全不发送；数值标量只进 behavior oneof；4xx（除 429）不重试不回队；
可重试失败回队由定时器补发（内存 cap max_queue_size）。
"""

from __future__ import annotations

import sys
import threading
from typing import Dict, List, Optional

from .config import SDK_VERSION, FlushResult, RiskEvent, TrackOptions, UbaConfig
from .json_wire import (build_behavior, build_event, envelope, parse_failed_count,
                        parse_flags)
from .session import SessionManager
from .transport import HttpResult, HttpTransport, UrllibTransport
from .types import ReportEvent
from .utils import merge_properties, to_rfc3339_ms, uuid_v4


class UbaClient:
    def __init__(self, config: UbaConfig,
                 transport: Optional[HttpTransport] = None,
                 session: Optional[SessionManager] = None) -> None:
        self.config = config
        self._transport = transport if transport is not None else UrllibTransport()
        self._session = session or SessionManager(config.session_timeout_ms)
        self._lock = threading.RLock()
        self._queue: List[Dict] = []
        self._flushing = False
        self._closed = False
        self._user_id: Optional[int] = None
        self._super_props: Dict[str, str] = {}
        self._flags: Dict[str, str] = {}
        self._exposed_flags: set = set()
        self._exposed_session: Optional[str] = None
        self._last_screen_at: Optional[int] = None
        self._user_agent = (
            f"uba-python/{SDK_VERSION} "
            f"({sys.platform})"
        )
        self._timer: Optional[threading.Timer] = None
        if config.flush_interval_ms > 0:
            self._start_timer()

    # ─── 上报 API ────────────────────────────────────────────────────────────

    def identify(self, user_id: int) -> None:
        """绑定用户 ID（服务端事件源里的用户）。0/None = 匿名。"""
        with self._lock:
            self._user_id = user_id or None

    def reset_user(self) -> None:
        with self._lock:
            self._user_id = None

    def set_super_properties(self, props: Dict[str, str]) -> None:
        """设置全局公共属性：合并进之后每个事件（同名覆盖）。"""
        with self._lock:
            self._super_props.update(props)

    def clear_super_properties(self) -> None:
        with self._lock:
            self._super_props.clear()

    def track(self, event_name: str,
              properties: Optional[Dict[str, str]] = None,
              options: Optional[TrackOptions] = None) -> None:
        """上报自定义行为事件（track_behavior 别名）。"""
        self.track_behavior(event_name, properties, options)

    def track_behavior(self, event_name: str,
                       properties: Optional[Dict[str, str]] = None,
                       options: Optional[TrackOptions] = None) -> None:
        event = self._build_behavior_event(event_name, properties, options)
        if event is None:
            return
        self._enqueue(event)

    def track_risk(self, event_name: str, risk: RiskEvent,
                   properties: Optional[Dict[str, str]] = None,
                   options: Optional[TrackOptions] = None) -> None:
        """上报风险事件（进入 uba_risk_events 通道）。"""
        if risk is None or not self._should_collect(event_name, options):
            return
        wire = build_event(
            "RISK", uuid_v4(), event_name, to_rfc3339_ms(),
            options, self._merged_properties(properties, options),
            self._user_id, self.config.resolved_device_id,
            self._current_session(options), self._platform(options),
            behavior=None, risk=risk,
        )
        self._enqueue(wire)

    def track_page_view(self, screen_name: str,
                        title: Optional[str] = None,
                        referer: Optional[str] = None) -> None:
        """页面浏览事件（服务端场景少用，供完整性保留）；自动携带上一页停留时长。"""
        import time
        now = int(time.time() * 1000)
        dwell = None
        if self._last_screen_at is not None:
            dwell = now - self._last_screen_at
        self._last_screen_at = now
        props: Dict[str, str] = {"pageUrl": screen_name}
        if title:
            props["title"] = title
        if referer:
            props["referer"] = referer
        opts = TrackOptions(
            event_category="navigation",
            object_type="page",
            object_id=screen_name,
            duration_ms=dwell if dwell is not None and dwell > 0 else None,
        )
        self.track_behavior("page_view", props, opts)

    def is_collecting(self) -> bool:
        """当前是否处于采集状态（未关闭、会话命中采样）。"""
        with self._lock:
            return not self._closed and self._session.in_sample(self.config.sample_rate)

    # ─── AB 实验开关 ──────────────────────────────────────────────────────────

    def load_flags(self) -> Dict[str, str]:
        """拉取最新实验开关并缓存。失败保留旧缓存并返回当前缓存。"""
        body = envelope_flags(self.config.app_id, self.config.app_secret,
                              self.config.resolved_device_id, self._user_id)
        res = self._transport.send(self.config.flags_url, body, self.config.timeout_ms)
        if res.is_ok:
            with self._lock:
                self._flags = parse_flags(res.body)
        else:
            self._debug(f"拉取 flags 失败：HTTP {res.status_code}")
        with self._lock:
            return dict(self._flags)

    def get_flag(self, key: str) -> Optional[str]:
        """读取开关变体；命中时自动上报一次 $feature_flag 曝光（每会话每开关一次）。"""
        with self._lock:
            variant = self._flags.get(key)
        if variant is not None:
            self._track_flag_exposure(key, variant)
        return variant

    def is_feature_enabled(self, key: str) -> bool:
        """变体非空且非 control 即视为命中（不触发曝光事件）。"""
        with self._lock:
            v = self._flags.get(key)
        return v is not None and v != "" and v != "control"

    # ─── 生命周期 & 维护 ──────────────────────────────────────────────────────

    def handle_background(self) -> FlushResult:
        """退到后台：记录时刻 + 立即 flush（客户端宿主接入用）。"""
        self._session.on_background()
        return self.flush()

    def handle_foreground(self) -> None:
        """回到前台：后台超过 session_timeout_ms 则轮换新会话。"""
        self._session.on_foreground()

    def pending_count(self) -> int:
        with self._lock:
            return len(self._queue)

    def flush(self) -> FlushResult:
        """手动冲刷队列：按批量/字节上限分批发送，直到清空或遇到失败。并发调用安全。"""
        with self._lock:
            if self._flushing or self._closed:
                return FlushResult(success=True)
            self._flushing = True

        sent = dropped = 0
        ok = True
        retryable = False
        message: Optional[str] = None
        try:
            while True:
                with self._lock:
                    if not self._queue:
                        break
                    batch = self._take_batch_locked()
                r = self._send_with_retry(batch)
                sent += r.sent
                dropped += r.dropped
                if not r.success:
                    ok = False
                    retryable = r.retryable
                    message = r.message
                    # 可重试失败（网络/5xx/429）回到队首等下次补发；确定性 4xx 直接丢弃
                    if retryable:
                        with self._lock:
                            self._queue[:0] = batch
                    break
        finally:
            with self._lock:
                self._flushing = False
        return FlushResult(success=ok, sent=sent, dropped=dropped,
                           retryable=retryable, message=message)

    def close(self) -> FlushResult:
        """停止定时器并冲刷剩余队列。之后 track 不再入队。"""
        self._stop_timer()
        result = self.flush()
        with self._lock:
            self._closed = True
        self._transport.close()
        return result

    # ─── 内部实现 ────────────────────────────────────────────────────────────

    def _start_timer(self) -> None:
        interval = self.config.flush_interval_ms / 1000.0
        if interval <= 0:
            return

        def tick() -> None:
            if self._closed:
                return
            if self.pending_count() > 0:
                self.flush()
            self._start_timer()  # 重新排程

        timer = threading.Timer(interval, tick)
        timer.daemon = True
        timer.name = "uba-flush"
        with self._lock:
            self._timer = timer
        timer.start()

    def _stop_timer(self) -> None:
        with self._lock:
            timer = self._timer
            self._timer = None
        if timer is not None:
            timer.cancel()

    def _should_collect(self, event_name: str, options: Optional[TrackOptions]) -> bool:
        if self._closed:
            self._debug(f"客户端已关闭，丢弃事件 {event_name}")
            return False
        return self._session.in_sample(self.config.sample_rate)

    def _current_session(self, options: Optional[TrackOptions]) -> str:
        if options is not None and options.session_id:
            return options.session_id
        if self.config.session_timeout_ms > 0:
            return self._session.check_timeout()
        return self._session.current

    def _platform(self, options: Optional[TrackOptions]) -> str:
        return (options.platform if options is not None else None) or self.config.platform

    def _build_behavior_event(self, event_name: str,
                              properties: Optional[Dict[str, str]],
                              options: Optional[TrackOptions]) -> Optional[Dict]:
        if not self._should_collect(event_name, options):
            return None
        if options is None or options.session_id is None:
            if self.config.session_timeout_ms > 0:
                self._session.check_timeout()
        behavior = build_behavior(options, os_name(), self.config.app_version,
                                  self.config.channel)
        wire = build_event(
            "BEHAVIOR", uuid_v4(), event_name, to_rfc3339_ms(),
            options, self._merged_properties(properties, options),
            self._user_id, self.config.resolved_device_id,
            self._current_session(options), self._platform(options),
            behavior=behavior, risk=None,
        )
        return wire

    def _merged_properties(self, properties: Optional[Dict[str, str]],
                           options: Optional[TrackOptions]) -> Optional[Dict[str, str]]:
        with self._lock:
            base = dict(self._super_props)
        return merge_properties(base, properties) if options is None or not options.properties \
            else merge_properties(merge_properties(base, properties), options.properties)

    def _enqueue(self, event: Dict) -> None:
        auto = False
        with self._lock:
            self._queue.append(event)
            if len(self._queue) > self.config.max_queue_size:
                del self._queue[: len(self._queue) - self.config.max_queue_size]
            auto = len(self._queue) >= self.config.batch_size
        if auto:
            threading.Thread(target=self._safe_flush, daemon=True, name="uba-auto-flush").start()

    def _safe_flush(self) -> None:
        try:
            self.flush()
        except Exception as e:  # pragma: no cover - 防止自动 flush 线程带异常退出
            self._debug(f"自动 flush 异常：{e}")

    def _take_batch_locked(self) -> List[Dict]:
        import json as _json
        batch: List[Dict] = []
        size = 0
        while self._queue and len(batch) < self.config.batch_size:
            encoded = len(_json.dumps(self._queue[0], ensure_ascii=False).encode("utf-8"))
            if batch and size + encoded > self.config.max_batch_bytes:
                break
            size += encoded
            batch.append(self._queue.pop(0))
        return batch

    def _send_with_retry(self, batch: List[Dict]) -> FlushResult:
        body = envelope(self.config.app_id, self.config.app_secret,
                        self._user_agent, batch)
        attempt = 0
        while True:
            res: HttpResult = self._transport.send(self.config.report_url, body,
                                                   self.config.timeout_ms)
            if res.is_ok:
                failed = parse_failed_count(res.body)
                if failed > 0:
                    self._debug(f"部分事件被服务端拒绝：failedCount={failed}")
                return FlushResult(success=True, sent=len(batch) - failed, dropped=failed)
            # 4xx（除 429）是确定性失败：整批丢弃，不重试
            if 400 <= res.status_code < 500 and res.status_code != 429:
                self._debug(f"HTTP {res.status_code}（不可重试），丢弃 {len(batch)} 条")
                return FlushResult(success=False, dropped=len(batch),
                                   message=f"HTTP {res.status_code}")
            if attempt >= max(self.config.max_retries, 0):
                reason = res.body if res.status_code == 0 else f"HTTP {res.status_code}"
                self._debug(f"重试耗尽（{reason}），{len(batch)} 条待下次补发")
                return FlushResult(success=False, dropped=len(batch), retryable=True,
                                   message=f"{reason} after {attempt + 1} attempts")
            delay = self.config.retry_base_delay_ms * (2 ** min(attempt, 30)) / 1000.0
            self._debug(f"HTTP {res.status_code}，{delay}s 后第 {attempt + 1} 次重试")
            import time as _time
            _time.sleep(delay)
            attempt += 1

    def _track_flag_exposure(self, key: str, variant: str) -> None:
        with self._lock:
            session_id = self._session.current
            if self._exposed_session != session_id:
                self._exposed_flags.clear()
                self._exposed_session = session_id
            first = key not in self._exposed_flags
            if first:
                self._exposed_flags.add(key)
        if first:
            self.track("$feature_flag",
                       {"flag_key": key, "variant": variant},
                       TrackOptions(event_category="experiment"))

    def _debug(self, message: str) -> None:
        if self.config.debug:
            print(f"[uba-python] {message}", file=sys.stderr)


def envelope_flags(app_id: str, app_secret: str, device_id: str,
                   user_id: Optional[int]) -> str:
    """flags 请求体：{appId, appSecret, deviceId, userId?}。"""
    import json as _json
    body: Dict = {
        "appId": app_id,
        "appSecret": app_secret,
        "deviceId": device_id,
    }
    if user_id:
        body["userId"] = user_id
    return _json.dumps(body, ensure_ascii=False)


def os_name() -> str:
    name = sys.platform
    if name.startswith("linux"):
        return "Linux"
    if name == "darwin":
        return "macOS"
    if name == "win32":
        return "Windows"
    return name
