# Agent Guide — sdk/wechat/uba

微信小程序/小游戏埋点 SDK（TypeScript）。权威契约在 `backend/api/protos/`；测试走 vitest + 手写 wx mock。

## 铁律

1. 契约单一真相源在后端 proto；camelCase 逐字段对齐 `json_name`，禁止手造字段。
2. `tenantId`/`serverTime`/`ip` 不发送；**数值标量（durationMs/quantity/score/level/sessionSeq）只进 behavior oneof**（golden.test.ts 锁定顶层键集合）。
3. behavior oneof 必须存在；risk 事件不得带 behavior。
4. 重试语义：4xx（除 429）不重试**不落盘**；可重试失败（网络/5xx/429）落盘（wx storage cap 500）补发。
5. 零运行时依赖：wx API 手写最小声明（src/wx-env.ts），不引 miniprogram-api-typings。
6. 默认值对齐其他端：批量 20/5s/3 次/1s 退避/8s 超时。
7. 环境探测：有 `Page` 全局 → `mini_program`，否则 `mini_game`；config.platform 可覆盖。

## 提交前自查

```bash
cd sdk/wechat/uba
pnpm typecheck   # tsc --noEmit 0 error
pnpm test        # vitest 全绿（金样/批量重试/落盘/包装器）
pnpm build       # 产物 dist/
```

> 本机 pnpm 需要 Node ≥ 22：用 scoop node 26 直跑 pnpm.mjs（`C:\Users\yangl\scoop\apps\nodejs\26.13.1\node.exe` + `C:\Users\yangl\scoop\persist\nodejs\bin\node_modules\pnpm\bin\pnpm.mjs`）。

改 types.ts/batcher.ts 序列化 → 必跑金样测试；改 App/Page 包装器 → 必跑 client.test.ts 的 instrumentApp/instrumentPage 用例。
