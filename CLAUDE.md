# CLAUDE.md — go-wind-uba(根)

本文件供 Claude / Claude Code 在本仓库工作时使用。
完整指南见同目录 `AGENTS.md`,两者保持同步;子目录各有更细的 `AGENTS.md` / `CLAUDE.md`。
下面是 Claude 最该先记住的要点,详细约定请读 `AGENTS.md`。

## 这是什么

**GoWind UBA** —— 企业级用户行为分析平台。SDK 采集 → collector 入库 → core 分析(Doris)→ admin 前端可视化。

## 仓库结构

```
backend/      Go 1.25 + Kratos + Ent + Wire(三服务:admin/collector/core)
frontend/
  admin/      vue-vben-admin v5 monorepo(Vue3 + TS,应用 @vben/web-antd)
  sdk/web/uba/  纯 TS 埋点 SDK
sdk/csharp/   C# SDK
docs/         架构/开发/SDK 文档
```

**进子目录工作时,以该子目录的 AGENTS.md 为准。**

## 三条全局铁律

1. **契约真相源在后端 proto。** `backend/api/protos/` → `make api` 同时生成后端 Go 与前端 TS(`frontend/admin/apps/admin/src/generated/api/`)。所有生成物禁手改。

2. **包管理器各就各位:** 后端 Go modules;前端 pnpm(锁 only-allow pnpm);C# 见其 README。

3. **分析模型是双端核心。** 改分析模型要同时看 `backend/app/core/`(Doris 查询)和 `frontend/admin/apps/admin/src/views/app/data-analysis/`。

## 数据流(必须理解)

```
SDK → collector(采集) → {PostgreSQL 业务元数据 | Doris 事件聚合}
                          → core(28+ 分析模型) → admin(SSE/REST 可视化)
```

- PG 存业务元数据(事务),Doris 存行为/做分析(OLAP),别混用。

## 找对文件

| 任务 | 先读 |
|---|---|
| 后端 API/模型/逻辑 | `backend/AGENTS.md` |
| 管理后台页面/i18n | `frontend/AGENTS.md` |
| 分析模型(双端) | `backend/AGENTS.md` + `frontend/AGENTS.md` |
| Web SDK | `frontend/AGENTS.md`(sdk/web/uba)|
| 整体架构 | `docs/architecture.md` + 根 `AGENTS.md` 第 4 节 |

## 常用入口

```bash
# 后端
cd backend && make help      # api / ent / wire / build / lint / test
# 前端
cd frontend/admin && pnpm install && pnpm dev:antd
# Web SDK
cd frontend/sdk/web/uba && pnpm build
```

详见根 `AGENTS.md` 及各子目录指南。
