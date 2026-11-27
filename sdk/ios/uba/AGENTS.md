# Agent Guide — sdk/ios/uba

iOS/macOS 原生埋点 SDK（Swift Package）。核心仅依赖 Foundation（URLSession + 手写 JSON writer），iOS 13+/macOS 10.15+ 通用。

## ⚠️ 验证环境

本仓库开发机为 Windows：**Swift 代码与 XCTest 已交付，编译/测试需在 macOS 跑 `swift build && swift test`**。改这个目录时在交付说明里明确标注验证状态，勿声称本机已验证。

## 铁律

1. 契约单一真相源在后端 proto；camelCase 逐字段对齐 `json_name`，禁止手造字段。
2. `tenantId`/`serverTime`/`ip` 不发送；**数值标量只进 behavior oneof**（UbaClientTests 金样断言整包每个数值键恰好一次）。
3. behavior oneof 必须存在；risk 事件不带 behavior。
4. 重试语义：4xx（除 429）不重试不落盘；可重试失败落盘（UserDefaults cap 500）补发。
5. **离线补发按 wire JSON 原样保存、恢复后直接重发（无损）**。
6. flush 是同步语义（信号量等待网络回调）：**重试调度不许用 client 串行队列**（会与 flush 的信号量等待死锁）——用全局队列 asyncAfter。
7. 零第三方依赖：URLSession + 手写 JSON writer；`HttpTransport`/`KeyValueStore` 可注入。
8. 默认值对齐其他端：批量 20/5s/3 次/1s 退避/8s 超时。

## 提交前自查（macOS）

```bash
cd sdk/ios/uba
swift build
swift test   # XCTest 全绿
```

改 UbaJson/UbaClient 序列化 → 必跑金样；改 flush 并发 → 检查第 6 条死锁约束。
