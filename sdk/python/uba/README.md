# UBA Python SDK

GoWind UBA 风行用户行为分析平台的 **Python 服务端埋点 SDK**（数据团队脚本、自动化任务、后端事件源）。
对接 collector `POST /uba/v1/report`，鉴权在请求 body（`appId` + `appSecret`），JSON camelCase，无 gzip，单批 < 1MiB。
**零第三方依赖**（仅标准库：`json`/`urllib`/`threading`），Python 3.9+。

## 结构

```
sdk/python/uba/
├── src/uba/
│   ├── client.py     # UbaClient：队列 + 定时/定量 flush + 重试退避 + AB 开关（线程安全）
│   ├── config.py     # UbaConfig/TrackOptions/RiskEvent/FlushResult（dataclass + validate）
│   ├── json_wire.py  # wire dict 构造（数值标量只进 behavior oneof；跳 None/空集合）
│   ├── transport.py  # HttpTransport 抽象 + urllib 默认实现（statusCode 0 = 网络失败）
│   ├── session.py    # 会话管理（前后台超时轮换 + 事件间隔超时轮换，时钟可注入）
│   ├── types.py      # ReportEvent 内部模型
│   └── utils.py      # uuid/RFC3339/FNV 采样哈希
└── tests/            # unittest：契约金样 + http.server 桩行为测试
```

## 快速开始

```bash
pip install -e sdk/python/uba   # 或直接把 src/uba 加进 PYTHONPATH
```

```python
from uba import UbaClient, UbaConfig, TrackOptions, RiskEvent

client = UbaClient(UbaConfig(
    app_id="your_app_id",
    app_secret="your_app_secret",
    endpoint="http://localhost:5700",
    app_version="1.0.0",
    debug=True,  # 联调期打开
))

client.identify(1001)
client.track("order_paid", {"orderId": "ORD-001"},
             TrackOptions(object_type="order", amount="99.90", op_result="success"))
client.track_risk("login_failed",
                  RiskEvent(risk_type="login_anomaly", risk_level="HIGH", risk_score=88.5))
client.close()  # 冲刷剩余队列并停止定时器
```

## 常用配置

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `batch_size` | `20` | 缓冲达到该数量触发 flush |
| `flush_interval_ms` | `5000` | 定时 flush 间隔；`<=0` 禁用 |
| `max_retries` | `3` | 失败重试次数 |
| `timeout_ms` | `8000` | 单次请求超时（须 < 服务端 10s） |
| `device_id` | hostname | 服务端实例标识 |
| `session_timeout_ms` | `0` | 会话超时轮换（0 = 进程级会话） |
| `sample_rate` | `1.0` | 会话级确定性采样 |

## 测试

```bash
cd sdk/python/uba
PYTHONPATH=src python -m unittest discover -s tests -v   # 25 个用例全绿
```

## 契约约束

- 字段 camelCase 与后端 proto `json_name` 逐一对齐，**禁止手造字段**
- `tenantId`/`serverTime`/`ip` 服务端补全不发送；数值标量只进 behavior oneof（金样锁定）
- 鉴权在 body；单批压在 1MiB 内；超时必须 < 服务端 10s
- 4xx（除 429）不重试不回队；可重试失败回队由定时器补发（内存 cap 10000 条）
- behavior oneof 必须存在（服务端校验 payload 与 eventType 匹配）
