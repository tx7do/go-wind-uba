# Agent Guide — sdk/web/uba

浏览器/Node 埋点 TS SDK（原 frontend/sdk/web/uba，已迁至仓库根 sdk/）。权威契约在后端 proto（backend/api/protos/）；接入文档见本目录 README.md 与根 docs/sdk_integration.md。

## 1. 是什么

纯 TS,`tsc` 构建(`pnpm build` → `dist/`),`typecheck` 跑 `tsc --noEmit`。职责:浏览器/Node 端采集用户行为,上报到 backend collector 服务。改动需同时兼顾:轻量、不阻塞宿主页面、网络容错(队列/重试)。

## 2. 铁律

1. 契约单一真相源在后端 proto；camelCase 逐字段对齐 `json_name`,禁止手造字段。
2. `tenantId`/`serverTime`/`ip` 不发送；**数值标量(durationMs/quantity/score/level)只进 behavior oneof**(各端金样测试锁定)。
3. 零依赖构建:纯 `tsc`,无 bundler;改契约时与本仓其余 10 端 SDK 同步(见 sdk/README.md 共同契约)。

## 3. 提交前自查

```bash
cd sdk/web/uba
pnpm typecheck   # tsc --noEmit 0 error
pnpm build       # 产物 dist/
```
