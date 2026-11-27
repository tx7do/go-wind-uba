# UBA iOS/macOS SDK（Swift）

GoWind UBA 风行用户行为分析平台的 **iOS / macOS 原生埋点采集 SDK**。
对接 collector `POST /uba/v1/report`，鉴权在请求 body（`appId` + `appSecret`），JSON camelCase，无 gzip，单批 < 1MiB。
Swift Package（SPM），核心仅依赖 Foundation（URLSession + 手写 JSON writer），iOS 13+ / macOS 10.15+ / tvOS / watchOS 通用。

> ⚠️ **验证环境**：本仓库开发机为 Windows，Swift 代码已按 SPM 结构交付并附带 XCTest，
> 编译与测试需在 macOS 上执行 `swift build && swift test`（本仓库 CI 未覆盖）。

## 结构

```
sdk/ios/uba/
├── Package.swift
├── Sources/UbaSDK/
│   ├── UbaClient.swift   # 客户端：队列 + 定时/定量 flush + 重试退避 + AB 开关（串行队列保护）
│   ├── UbaConfig.swift   # 配置（precondition 校验，默认值对齐其他端）
│   ├── UbaModels.swift   # RiskEvent/TrackOptions/BehaviorPayload + RFC3339/FNV 工具
│   ├── UbaJson.swift     # 手写零依赖 JSON writer（跳 null/空集合）
│   └── （同目录）KeyValueStore / HttpTransport / SessionManager 定义在 UbaClient.swift
└── Tests/UbaSDKTests/    # XCTest：契约金样 + StubTransport 行为测试（重试/落盘/恢复）
```

## 特性

- ✅ 批量上报 + 指数退避重试；4xx（除 429）不重试不落盘；可重试失败落盘补发（UserDefaults，cap 500）
- ✅ 会话轮换（后台 30s 超时）+ 会话级确定性采样 + consent 门控
- ✅ 首装 `app_launch` + `trackPageView`（含停留时长与来源页）
- ✅ AB 开关 + `$feature_flag` 曝光去重
- ✅ 离线补发按 wire JSON 原样保存（无损重发）；`HttpTransport`/`KeyValueStore` 均可注入替换
- ✅ 不采集 IDFA（如需自实现 Context 注入）

## 快速开始

```swift
// Package.swift 依赖
dependencies: [
    .package(path: "../go-wind-uba/sdk/ios/uba")
]
// 或 Xcode：File > Add Packages > Add Local...
```

```swift
import UbaSDK

let client = UbaClient(config: UbaConfig(
    appId: "your_app_id",
    appSecret: "your_app_secret",
    endpoint: "http://collector.example.com:5700",
    appVersion: "1.0.0",
    autoTrackLaunch: true,
    debug: true
))

// AppDelegate 前后台接入
func applicationDidEnterBackground(_ application: UIApplication) {
    client.handleBackground() // 退后台立即 flush
}
func applicationWillEnterForeground(_ application: UIApplication) {
    client.handleForeground() // 超时轮换会话
}

// 业务埋点
client.identify(1001)
client.track("button_click", properties: ["scene": "home"],
             options: TrackOptions(objectType: "button", objectId: "btn_buy"))
client.trackRisk("abnormal_click", RiskEvent(
    riskType: "device_anomaly", riskLevel: "HIGH", riskScore: 85
))
client.trackPageView("HomeViewController")
```

## 常用配置

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `batchSize` | `20` | 缓冲达到该数量触发 flush |
| `flushIntervalMs` | `5000` | 定时 flush 间隔；0 禁用 |
| `maxRetries` | `3` | 失败重试次数 |
| `timeoutMs` | `8000` | 单次请求超时（须 < 服务端 10s） |
| `sessionTimeoutMs` | `30000` | 后台超时轮换会话 |
| `sampleRate` | `1.0` | 会话级确定性采样 |
| `requireConsent` | `false` | 隐私合规授权门控 |

## 构建（macOS）

```bash
cd sdk/ios/uba
swift build
swift test     # XCTest 全绿（契约金样 + StubTransport 行为）
```

## 契约约束

- 字段 camelCase 与后端 proto `json_name` 逐一对齐，**禁止手造字段**
- `tenantId`/`serverTime`/`ip` 服务端补全不发送；数值标量（durationMs/quantity/score/level）只进 behavior oneof
- 鉴权在 body；单批压在 1MiB 内；超时必须 < 服务端 10s（默认 8s）
- 4xx（除 429）不重试不落盘；HTTP 200 仍可能有单条部分失败（`failedCount`）
