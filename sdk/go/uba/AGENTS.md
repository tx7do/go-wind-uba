# Agent Guide — sdk/go/uba

Go 服务端埋点 SDK（服务端事件源：支付/订单/风控）。权威契约在 `backend/api/protos/`（collector i_report + uba report/behavior_event/risk_event）。

## 铁律

1. 契约单一真相源在后端 proto；字段名逐一对齐 `json_name`，禁止手造字段。
2. `tenantId`/`serverTime`/`ip` 不发送（types.go 结构体里就没有这些字段）。
3. **数值标量（durationMs/quantity/score/level/sessionSeq）只进 behavior oneof**（proto3 无法区分 unset 0，顶层不设字段——client_test 金样断言顶层键集合）。
4. behavior oneof 必须存在（服务端校验 payload 与 eventType 匹配）；无选项事件也带空载荷（os/appVersion）。
5. 重试语义：4xx（除 429）不重试且**不回队**；网络/5xx/429 耗尽重试回队由定时器补发（内存 cap 10000 条）。
6. 零第三方依赖（仅标准库）；`HTTPClient` 接口可注入，不改 client.go。
7. 默认值与其他端对齐：批量 20/5s/3 次/1s 退避/8s 超时（<10s 校验）。

## 提交前自查

```bash
cd sdk/go/uba
go vet ./...
go test ./... -count=1 -race   # 全绿 + race 干净
```

改 types.go 序列化 → 必跑契约金样（utils_test.go TestEventJSONContract 断言顶层键集合精确）。
