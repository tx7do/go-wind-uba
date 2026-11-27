# Agent Guide — sdk/java/uba

Java 服务端埋点 SDK（JDK 11+，Maven 构建）。权威契约在 `backend/api/protos/`。

## 铁律

1. 契约单一真相源在后端 proto；camelCase 逐字段对齐 `json_name`，禁止手造字段。
2. `tenantId`/`serverTime`/`ip` 不发送；**数值标量（durationMs/quantity/score/level）只进 behavior oneof**（UbaClientTest 金样断言整包 JSON 中每个数值键恰好出现一次）。
3. behavior oneof 必须存在；risk 事件不得带 behavior。
4. 重试语义：4xx（除 429）不重试**不回队**；可重试失败（网络/5xx/429）回队由定时器补发（内存 cap 10000）。
5. 零第三方依赖：JDK HttpClient + 手写 JsonWriter（禁引 Jackson/Gson 等运行时依赖）。
6. `maven.compiler.release=11`：禁用 Java 14+ 语法（如 switch 箭头表达式）。
7. 默认值对齐其他端：批量 20/5s/3 次/1s 退避/8s 超时（validate 校验 <10s）。

## 提交前自查

```bash
cd sdk/java/uba
mvn test      # JUnit 5 全绿
mvn package
```

改 JsonWriter/UbaClient 构造 → 必跑金样（goldenBehaviorEvent 断言键出现次数与顺序敏感字段）。
