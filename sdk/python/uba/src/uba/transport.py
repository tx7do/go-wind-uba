"""HTTP 传输抽象 + urllib 默认实现（statusCode 0 = 网络层失败，可重试）。"""

from __future__ import annotations

import urllib.error
import urllib.request
from abc import ABC, abstractmethod
from dataclasses import dataclass
from typing import Optional

@dataclass
class HttpResult:
    """传输结果。statusCode 0 表示网络层失败（超时/连接拒绝），可重试。"""

    status_code: int
    body: str

    @property
    def is_ok(self) -> bool:
        return 200 <= self.status_code < 300


class HttpTransport(ABC):
    """HTTP 传输抽象：默认 urllib；换 requests/网关/加签时实现本接口注入。"""

    @abstractmethod
    def send(self, url: str, body: str, timeout_ms: int) -> HttpResult:
        ...

    def close(self) -> None:  # pragma: no cover
        """释放底层资源（默认无连接池）。"""


class UrllibTransport(HttpTransport):
    """标准库 urllib 实现（零第三方依赖）。"""

    def send(self, url: str, body: str, timeout_ms: int) -> HttpResult:
        timeout_s = max(timeout_ms, 1) / 1000.0
        req = urllib.request.Request(
            url,
            data=body.encode("utf-8"),
            method="POST",
            headers={"Content-Type": "application/json"},
        )
        try:
            with urllib.request.urlopen(req, timeout=timeout_s) as resp:
                text = resp.read().decode("utf-8", errors="replace")
                return HttpResult(resp.status, text)
        except urllib.error.HTTPError as e:  # 4xx/5xx 也走 HTTPError
            try:
                text = e.read().decode("utf-8", errors="replace")
            except OSError:  # pragma: no cover
                text = ""
            return HttpResult(e.code, text)
        except (urllib.error.URLError, TimeoutError, OSError) as e:
            return HttpResult(0, str(e))
