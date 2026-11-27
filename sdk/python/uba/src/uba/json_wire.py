"""wire JSON 构造：事件模型 → camelCase dict（跳 None/空集合），由 json.dumps 序列化。

契约要点（对齐全仓库 SDK）：
- 顶层字符串镜像维度 + behavior oneof 冗余（对齐 Web/C#/Flutter 写法）；
- 数值标量（durationMs/quantity/score/sessionSeq/serverId/level）只进 behavior（proto3 顶层无法区分 unset 0）；
- tenantId/serverTime/ip 由服务端补全，不构造。
"""

from __future__ import annotations

import json
from typing import Any, Dict, List, Optional

from .config import RiskEvent, TrackOptions
from .types import ReportEvent


def _put(m: Dict[str, Any], key: str, value: Any) -> None:
    """写入非 null 字段；空 dict/空 list 跳过（对齐其他端 putIfPresent 语义）。"""
    if value is None:
        return
    if isinstance(value, (dict, list)) and not value:
        return
    m[key] = value


def risk_to_wire(r: Optional[RiskEvent]) -> Optional[Dict[str, Any]]:
    if r is None:
        return None
    m: Dict[str, Any] = {}
    _put(m, "riskType", r.risk_type)
    _put(m, "riskLevel", r.risk_level)
    _put(m, "riskScore", r.risk_score)
    _put(m, "ruleId", r.rule_id)
    _put(m, "ruleName", r.rule_name)
    _put(m, "ruleContext", r.rule_context)
    _put(m, "relatedEventIds", r.related_event_ids)
    _put(m, "description", r.description)
    _put(m, "evidence", r.evidence)
    _put(m, "status", r.status)
    return m or None


def build_behavior(o: Optional[TrackOptions], os_name: str, app_version: str,
                   channel: str) -> Dict[str, Any]:
    """behavior oneof 载荷构造：数值标量只在这里才被服务端信任。"""
    b: Dict[str, Any] = {"os": os_name}
    if app_version:
        b["appVersion"] = app_version
    if channel:
        b["channel"] = channel
    if o is not None:
        _put(b, "eventAction", o.event_action)
        _put(b, "objectType", o.object_type)
        _put(b, "objectId", o.object_id)
        _put(b, "objectName", o.object_name)
        _put(b, "network", o.network)
        _put(b, "durationMs", o.duration_ms)
        _put(b, "amount", o.amount)
        _put(b, "quantity", o.quantity)
        _put(b, "score", o.score)
        _put(b, "metrics", o.metrics)
        _put(b, "opResult", o.op_result)
        _put(b, "errorCode", o.error_code)
        _put(b, "serverId", o.server_id)
        _put(b, "level", o.level)
    return b


def build_event(event_type: str, event_id: str, event_name: str, event_time: str,
                o: Optional[TrackOptions], properties: Optional[Dict[str, str]],
                user_id: Optional[int], device_id: str, session_id: str,
                platform: str, behavior: Optional[Dict[str, Any]],
                risk: Optional[RiskEvent]) -> Dict[str, Any]:
    """顶层事件构造。数值标量不落顶层（proto3 顶层无法区分 unset 0，服务端只信 oneof 内的值）。"""
    m: Dict[str, Any] = {
        "eventType": event_type,
        "eventId": event_id,
        "eventName": event_name,
        "eventTime": event_time,
    }
    uid = o.user_id if (o is not None and o.user_id is not None) else user_id
    _put(m, "userId", uid)
    _put(m, "deviceId", (o.device_id if o is not None else None) or device_id)
    _put(m, "sessionId", (o.session_id if o is not None else None) or session_id)
    _put(m, "platform", (o.platform if o is not None else None) or platform)
    _put(m, "eventCategory", o.event_category if o is not None else None)
    _put(m, "properties", properties)
    _put(m, "metrics", o.metrics if o is not None else None)
    _put(m, "traceId", o.trace_id if o is not None else None)
    _put(m, "eventAction", o.event_action if o is not None else None)
    _put(m, "objectType", o.object_type if o is not None else None)
    _put(m, "objectId", o.object_id if o is not None else None)
    _put(m, "objectName", o.object_name if o is not None else None)
    _put(m, "amount", o.amount if o is not None else None)
    _put(m, "opResult", o.op_result if o is not None else None)
    _put(m, "errorCode", o.error_code if o is not None else None)
    if risk is not None:
        _put(m, "risk", risk_to_wire(risk))
    elif behavior:
        _put(m, "behavior", behavior)
    return m


def envelope(app_id: str, app_secret: str, user_agent: Optional[str],
             events: List[Dict[str, Any]]) -> str:
    """上报 envelope：{appId, appSecret, clientInfo?, events[]}。events 为 wire dict。"""
    env: Dict[str, Any] = {"appId": app_id, "appSecret": app_secret}
    if user_agent:
        env["clientInfo"] = {"userAgent": user_agent}
    env["events"] = events
    return json.dumps(env, ensure_ascii=False, separators=(",", ":"))


def parse_failed_count(body: str) -> int:
    """从 {"success":true,"failedCount":2} 提取 failedCount。"""
    try:
        parsed = json.loads(body) if body else {}
        v = parsed.get("failedCount", 0)
        return int(v) if isinstance(v, (int, float)) else 0
    except (ValueError, TypeError):
        return 0


def parse_flags(body: str) -> Dict[str, str]:
    """从 {"flags":{"k":"v"}} 提取键值对。"""
    try:
        parsed = json.loads(body) if body else {}
        flags = parsed.get("flags")
        if isinstance(flags, dict):
            return {str(k): str(v) for k, v in flags.items()}
    except (ValueError, TypeError):
        pass
    return {}
