# Frontend Agent Guide — go-wind-uba

本文件是 AI 编码助手(以及新人)在本目录 `frontend/` 下工作时的权威指南。
请先通读,再动手。

## 1. 技术栈与项目布局

`frontend/` 下有两个独立的前端产物:

| 子目录 | 技术栈 | 说明 |
|---|---|---|
| `admin/` | Vue 3 + TypeScript + Vite + Ant Design Vue,基于 [vue-vben-admin v5](https://github.com/vben/vue-vben-admin) monorepo | 管理后台(主前端) |
| `sdk/web/uba/` | 纯 TypeScript SDK,`tsc` 编译 | 浏览器/Node 埋点 SDK,上报到 collector 服务 |

两者均使用 **pnpm**(admin 有 `preinstall: only-allow pnpm`,**禁止 npm/yarn**)。

## 2. `admin/` —— vue-vben-admin monorepo

### 包管理
- 用 **pnpm + workspace + turbo + changeset**。
- 包名前缀:`@vben/*`(内部包)、`@vben-core/*`(核心无样式原语)。
- 应用入口:`apps/admin`(`@vben/web-antd`)。
- 共享包:`packages/{@core,constants,effects,icons,locales,preferences,stores,styles,types,utils}`。
- 内部工具:`internal/{lint-configs,node-utils,tailwind-config,tsconfig,vite-config}`。
- 依赖版本统一用 `catalog:`(见根 `package.json` 的 `pnpm-workspace.yaml` catalog)。

### 关键脚本(在 `admin/` 下执行)
```bash
pnpm install            # 安装(catalog 解析依赖 workspace)
pnpm dev                # 选 app 跑(turbo-run dev)
pnpm dev:antd           # 直接跑 @vben/web-antd
pnpm build              # NODE_OPTIONS=--max-old-space-size=8192 turbo build
pnpm check              # 循环依赖 + 依赖 + 类型 + 拼写 全套检查
pnpm check:type         # turbo run typecheck (vue-tsc)
pnpm lint               # vsh lint
pnpm format             # vsh lint --format
pnpm test:unit          # vitest run --dom
```
> 内存上限:build 显式给到 8GB,本机调试慢时注意。

### 应用内目录(`apps/admin/src/`)
```
adapter/        组件适配层
api/            业务 API 封装(Composable)
generated/      后端 proto 自动生成的调用代码($/*) —— 禁止手改
  └── api/      由 backend make api 生成
transport/
  ├─ rest/      REST 请求客户端(request-client / 拦截器 / 分页)
  └─ sse/       Server-Sent Events 客户端(实时分析流)
layouts/  router/  stores/  plugins/  locales/  constants/  utils/
views/
  ├─ app/       业务主区
  │   └─ data-analysis/   ★ 数据分析核心(20+ 分析模型)
  ├─ dashboard/ 看板
  ├─ _core/     认证/异常/about 等框架页
  ├─ message/  profile/
```

### `data-analysis/` —— 核心业务(28 个分析模型)
分析模型是本产品的核心,目录即模型:`anomaly / attribution / behavior-sequence /
churn / click / dimension-compare / distribution / economy / event-trend / funnel /
interval / level-analysis / lifecycle / ltv / matrix / new-vs-old / online-stats /
path-sankey / realtime-screen / retention / revenue / segmentation / session /
session-analysis / server-retention / whale-tier`。
还含共享 `components/`。改动分析页时优先复用这些组件与统一的请求/图表封装。

## 3. `sdk/web/uba/` —— 埋点 SDK

- 纯 TS,`tsc` 构建(`pnpm build` → `dist/`),`typecheck` 跑 `tsc --noEmit`。
- 职责:浏览器/Node 端采集用户行为,上报到 backend `collector` 服务。
- 改动需同时兼顾:轻量、不阻塞宿主页面、网络容错(队列/重试)。

## 4. 必须遵守的约定

1. **包管理器锁死 pnpm。** admin 设了 `only-allow pnpm`,别引入 npm/yarn 锁文件。
2. **生成代码不可手改。** `apps/admin/src/generated/api/` 来自后端 proto(`make api`)。改 API 先改后端 proto 再重新生成。
3. **跨包引用用 workspace 协议。** `workspace:*`,版本走 `catalog:`。新增第三方依赖前先看 catalog 是否已收录。
4. **类型优先。** 全 TS,`pnpm check:type` 必过;API 返回类型以 `generated/api` 为准,不要手抄。
5. **i18n 必须维护。** `locales/langs/{zh-CN,en-US}` 双语,新文案两份都要加。后端错误码到文案的映射在常量层统一处理。
6. **实时数据走 SSE。** 实时分析场景用 `transport/sse/`,不要用轮询。
7. **提交前自查:**
   - `pnpm check:type` 通过
   - `pnpm lint` 无新增报错
   - 新增依赖已在 catalog/workspace 内正确声明
   - 文案已补全双语

## 5. 前后端协作

- 后端 proto 改动 → `make api`(在 backend)会同时生成前端的 TypeScript HTTP 调用代码到 `frontend/admin/apps/admin/src/generated/api/`。
- 因此**改契约是后端发起、前端被动接收**,前端不要手动编辑 `generated/`。
- 若生成结构与预期不符,去后端 `api/protos/` 或生成插件配置里排查,而不是改前端生成物。

## 6. 调试提示

- `pnpm dev` 启不来 → 先 `pnpm install`,确认 workspace 与 catalog 解析正常;再确认 `internal/*` 的 stub(`postinstall: pnpm -r run stub`)跑过。
- 类型报错引用不到包 → 多半是 workspace 没正确 link,或新包没加到 `pnpm-workspace.yaml`。
- 构建慢/OOM → 已给 8GB 上限,仍不够说明有大对象/循环,跑 `pnpm check:circular`。
- 实时页无数据 → 查 `transport/sse/` 连接与后端 collector/core SSE 端点。
