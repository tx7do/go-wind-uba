# CLAUDE.md — frontend/sdk/wechat/uba 速记

- 微信小程序/小游戏 TS SDK，零运行时依赖（wx API 手写声明在 src/wx-env.ts）。
- 宿主探测：有 `Page` 全局 → mini_program，否则 mini_game；config.platform 可覆盖。
- camelCase、body 鉴权、无 gzip、单批 <1MiB、超时 8s；`tenantId`/`serverTime`/`ip` 不发。
- 数值标量只进 behavior oneof；behavior oneof 必须存在。
- 重试：1s×2^n×3；4xx(除429) 不重试不落盘；可重试失败落盘（wx storage cap 500）补发。
- `instrumentApp`/`instrumentPage` 自动埋点（onHide flush、onLoad page_view）。
- 提交前：`pnpm typecheck && pnpm test && pnpm build`（Node≥22，scoop node 26 直跑 pnpm.mjs）。
