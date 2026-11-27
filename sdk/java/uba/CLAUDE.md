# CLAUDE.md — sdk/java/uba 速记

- Java 服务端 SDK，零依赖（JDK11 HttpClient + 手写 JsonWriter）；deviceId=hostname，platform=`server`。
- camelCase、body 鉴权、无 gzip、单批 <1MiB、超时 8s；`tenantId`/`serverTime`/`ip` 不发。
- 数值标量只进 behavior oneof；behavior oneof 必须存在。
- 重试：1s×2^n×3；4xx(除429) 不重试不回队；可重试耗尽回队由定时器补发（cap 10000）。
- `maxRetries` 0=默认3、负数=禁用；`flushInterval=Duration.ZERO` 禁用定时器。
- 提交前：`mvn test`（surefire JUnit5）。
