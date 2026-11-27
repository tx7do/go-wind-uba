# CLAUDE.md — sdk/android/uba 速记

- 分层：核心纯 Kotlin（JVM 可测）+ `android/` 适配层源码（宿主工程编译，本仓库不编译）。
- camelCase、body 鉴权、无 gzip、单批 <1MiB、超时 8s；`tenantId`/`serverTime`/`ip` 不发。
- 数值标量只进 behavior oneof；behavior oneof 必须存在。
- 重试：1s×2^n×3；4xx(除429) 不重试不落盘；可重试失败落盘（SharedPreferences cap 500）补发。
- 离线补发存 wire JSON 原样重发（无损），勿改回反序列化方案。
- 提交前：`gradle test`（核心层）。
