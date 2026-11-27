# UBA Java SDK

GoWind UBA 风行用户行为分析平台的 **Java 服务端埋点 SDK**（订单/支付/风控等后端事件源）。
对接 collector `POST /uba/v1/report`，鉴权在请求 body（`appId` + `appSecret`），JSON camelCase，无 gzip，单批 < 1MiB。
**零第三方依赖**（JDK 11+ 内置 HttpClient + 手写 JSON writer），仅测试引入 JUnit 5。

## 结构

```
sdk/java/uba/
├── src/main/java/com/gowind/uba/
│   ├── UbaClient.java     # 客户端：队列 + 定时/定量 flush + 重试退避 + AB 开关（线程安全）
│   ├── UbaConfig.java     # 流式配置 + validate 校验（默认值对齐其他端）
│   ├── TrackOptions.java  # 单次上报维度 + TrackOptions.p(k,v,...) 属性便捷构造
│   ├── RiskEvent.java     # 风险事件载荷（risk oneof）
│   ├── FlushResult.java   # flush 结果
│   ├── HttpTransport.java # 传输抽象 + JDK HttpClient 默认实现（statusCode 0 = 网络失败）
│   └── JsonWriter.java    # 手写零依赖 JSON（跳 null/空集合，转义齐全）
└── src/test/java/         # JUnit 5：契约金样 + com.sun.net.httpserver 桩服务行为测试
```

## 快速开始

```xml
<dependency>
  <groupId>com.gowind</groupId>
  <artifactId>uba-sdk</artifactId>
  <version>1.0.0</version>
</dependency>
```

```java
import com.gowind.uba.*;

UbaClient client = new UbaClient(new UbaConfig()
        .appId("your_app_id")
        .appSecret("your_app_secret")
        .endpoint("http://localhost:5700")
        .appVersion("1.0.0")
        .debug(true)); // 联调期打开

client.identify(1001);
client.track("order_paid", TrackOptions.p("orderId", "ORD-001"),
        new TrackOptions().objectType("order").amount("99.90").opResult("success"));

client.trackRisk("login_failed",
        new RiskEvent().riskType("login_anomaly").riskLevel("HIGH").riskScore(88.5),
        TrackOptions.p("try_count", "8"), null);

client.close(); // 冲刷剩余队列并停止定时器
```

## 常用配置

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `batchSize` | `20` | 缓冲达到该数量触发 flush |
| `flushInterval` | `5s` | 定时 flush 间隔；`Duration.ZERO` 禁用 |
| `maxRetries` | `3` | 失败重试次数；负数禁用重试 |
| `timeout` | `8s` | 单次请求超时（须 < 服务端 10s） |
| `deviceId` | hostname | 服务端实例标识 |
| `sessionTimeout` | `0` | 会话超时轮换（0 = 进程级会话） |
| `sampleRate` | `1` | 会话级确定性采样 |

## 构建

```bash
cd sdk/java/uba
mvn test        # JUnit 5 全绿（契约金样 + 重试/落盘/部分失败）
mvn package     # 产物 target/uba-sdk-1.0.0.jar
```

## 契约约束

- 字段 camelCase 与后端 proto `json_name` 逐一对齐，**禁止手造字段**
- `tenantId`/`serverTime`/`ip` 服务端补全不发送；数值标量（durationMs/quantity/score/level）只进 behavior oneof
- 鉴权在 body；单批压在 1MiB 内；超时必须 < 服务端 10s
- 4xx（除 429）不重试不回队；可重试失败回队由定时器补发（内存 cap 10000 条）
- behavior oneof 必须存在（服务端校验 payload 与 eventType 匹配）
