# CLAUDE.md — sdk/ios/uba 速记

- Swift Package；核心仅 Foundation（URLSession + 手写 JSON writer）；**本机 Windows 不验证，macOS 跑 swift test**。
- camelCase、body 鉴权、无 gzip、单批 <1MiB、超时 8s；`tenantId`/`serverTime`/`ip` 不发。
- 数值标量只进 behavior oneof；behavior oneof 必须存在。
- 重试：1s×2^n×3；4xx(除429) 不重试不落盘；可重试失败落盘（UserDefaults cap 500）补发（wire JSON 原样）。
- flush 同步（信号量）；重试调度必须走全局队列（client 串行队列会死锁）。
- 平台标识：iOS→`ios`，macOS/Catalyst→`macos`。
