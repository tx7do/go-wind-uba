# CLAUDE.md — sdk/harmonyos/uba 速记

- ArkTS hvigor library；**Windows 本机不验证，DevEco 跑 hvigorw test**。
- camelCase、body 鉴权、无 gzip、单批 <1MiB、超时 8s；`tenantId`/`serverTime`/`ip` 不发。
- 数值标量只进 behavior oneof；behavior oneof 必须存在。
- 重试：1s×2^n×3；4xx(除429) 不重试；可重试失败回队补发。
- ArkTS 严格语法：无 any/解构/spread；putField 辅助构造 wire 对象。
- module.json5 已申请 ohos.permission.INTERNET。
