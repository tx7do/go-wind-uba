# UBA Go SDK

GoWind UBA 风行用户行为分析平台的 **Go 服务端埋点 SDK**，用于后端服务发事件（支付回调、风控触发、订单流转等服务端场景）。
对接 collector `POST /uba/v1/report`，鉴权在请求 body（`appId` + `appSecret`），JSON camelCase，无 gzip，单批 < 1MiB。**零第三方依赖**（仅标准库）。

## 结构

```
sdk/go/uba/
├── client.go    # UbaClient：队列 + 定时/定量 flush + 重试退避 + AB 开关（并发安全）
├── config.go    # Config + TrackOptions（默认值对齐其他端：批量 20/5s/3 次重试/1s 退避/8s 超时）
├── types.go     # Event/behaviorPayload/riskRiskEvent + envelope（对齐 proto json_name）
├── utils.go     # uuid v4 / RFC3339 毫秒 / FNV 采样哈希
└── *_test.go    # 契约金样 + httptest 行为测试（重试/落盘/并发 race）
```

## 特性

- ✅ 服务端语义：deviceId 取实例 hostname（可配），platform 默认 `server`，无设备/会话概念可覆写
- ✅ 批量上报（20 条或 5s 定时触发）+ 指数退避重试（1s×2^n，最多 3 次；4xx 除 429 不重试）
- ✅ 可重试失败（网络/5xx/429）留在内存队列由定时器补发（cap 10000 条防无界增长）；4xx 确定性失败直接丢弃
- ✅ AB 实验开关（`/uba/v1/flags`）+ `$feature_flag` 曝光事件（每会话每开关一次）
- ✅ 并发安全（`go test -race` 验证）；`HTTPClient` 接口可注入自定义传输（代理/网关/加签）
- ✅ 会话超时轮换可选（默认进程级会话，`SessionTimeout > 0` 时按事件间隔判定）

## 快速开始

```bash
go get go-wind-uba/sdk/go/uba   # 或以相对 module 引入（见 go.mod）
```

```go
package main

import (
    "time"
    "go-wind-uba/sdk/go/uba"
)

func main() {
    client, err := uba.NewClient(uba.Config{
        AppID:     "your_app_id",
        AppSecret: "your_app_secret",
        Endpoint:  "http://localhost:5700",
        AppVersion: "1.0.0",
        Debug:     true, // 联调期打开
    })
    if err != nil {
        panic(err)
    }
    defer client.Close()

    client.Identify(1001)
    client.Track("order_paid", uba.P{"orderId": "ORD-001"}, &uba.TrackOptions{
        EventCategory: "order",
        ObjectType:    "order",
        ObjectID:      "ORD-001",
        Amount:        "99.90",
        OpResult:      "success",
    })
    client.TrackRisk("login_failed", &uba.RiskEvent{
        RiskType:  "login_anomaly",
        RiskLevel: "HIGH",
        RiskScore: 88.5,
    }, uba.P{"try_count": "8"}, nil)
}
```

## 常用配置

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `BatchSize` | `20` | 缓冲达到该数量触发 flush |
| `FlushInterval` | `5s` | 定时 flush 间隔；负数禁用定时器 |
| `MaxRetries` | `3` | 失败重试次数；负数禁用重试 |
| `Timeout` | `8s` | 单次请求超时（须 < 服务端 10s） |
| `DeviceID` | hostname | 服务端实例标识 |
| `SessionTimeout` | `0` | 会话超时轮换（0 = 进程级会话） |
| `SampleRate` | `1` | 会话级确定性采样 |

## 构建

```bash
cd sdk/go/uba
go vet ./...
go test ./... -count=1 -race   # 全绿 + race 干净
```

## 契约约束

- 字段 camelCase 与后端 proto `json_name` 逐一对齐，**禁止手造字段**
- `tenantId`/`serverTime`/`ip` 服务端补全，SDK 结构体里就没有这些字段
- 数值标量（durationMs/quantity/score/level）**只进 behavior oneof**（proto3 无法区分 unset 0）
- 鉴权在 body，无 Authorization header；单批请求体压在 1MiB 内（服务端对未鉴权大包直接 401）
- HTTP 200 仍可能有单条部分失败（`failedCount`），按丢弃计
- behavior oneof 必须存在（服务端校验 payload 与 eventType 匹配），无选项的事件也会带空载荷（os/appVersion）
