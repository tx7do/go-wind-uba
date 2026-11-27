# Agent Guide — sdk/python/uba

Python 服务端埋点 SDK（Python 3.9+，零第三方依赖）。权威契约在 `backend/api/protos/`。

## 铁律

1. 契约单一真相源在后端 proto；camelCase 逐字段对齐 `json_name`，禁止手造字段。
2. `tenantId`/`serverTime`/`ip` 不发送；**数值标量（durationMs/quantity/score/level）只进 behavior oneof**（test_golden.py 锁定顶层键集合精确）。
3. behavior oneof 必须存在；risk 事件不带 behavior。
4. 重试语义：4xx（除 429）不重试不回队；可重试失败（网络/5xx/429）回队由定时器补发（内存 cap 10000）。
5. 零第三方依赖：标准库 json/urllib/threading；`HttpTransport`/时钟可注入。
6. 默认值对齐其他端：批量 20/5s/3 次/1s 退避/8s 超时（`__post_init__` 校验）。
7. Pythonic：配置/选项用 dataclass 蛇形命名，wire 层转驼峰（json_wire.py 集中管理键名）。

## 提交前自查

```bash
cd sdk/python/uba
PYTHONPATH=src python -m unittest discover -s tests -v   # 全绿
```

改 json_wire.py/client.py 序列化 → 必跑金样（test_golden.py 断言顶层键集合精确）。
