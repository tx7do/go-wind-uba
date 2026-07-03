# CLAUDE.md — go-wind-uba frontend

本文件供 Claude / Claude Code 在 `frontend/` 目录工作时使用。
权威与完整指南见同目录 `AGENTS.md`;两者保持同步,内容等价。
下面是 Claude 最该先记住的要点,详细约定请读 `AGENTS.md`。

## 你在哪里

`go-wind-uba` 前端,两个独立产物:
- `admin/` —— Vue 3 + TS + Vite + Ant Design Vue,vue-vben-admin v5 的 pnpm monorepo(应用 `@vben/web-antd`)。
- `sdk/web/uba/` —— 纯 TS 埋点 SDK,上报到后端 collector。

## 三条铁律

1. **只能用 pnpm。** admin 有 `only-allow pnpm` 锁;workspace 内部包用 `workspace:*`,版本用 `catalog:`。

2. **生成代码不可手改。** `apps/admin/src/generated/api/` 来自后端 proto(`backend: make api`)。改 API 先改后端 proto 再重新生成,前端被动接收。

3. **类型 + i18n 必须维护。**
   - `pnpm check:type`(`vue-tsc`)必过,API 类型以 `generated/api` 为准。
   - 新增文案 `locales/langs/{zh-CN,en-US}` 两份都要补。

## 目录速记

```
admin/
  apps/admin/src/
    generated/api/  后端生成(禁改)
    transport/{rest,sse}/  请求客户端;实时分析走 SSE
    views/app/data-analysis/  ★ 28 个分析模型(核心业务)
  packages/  @vben/* 共享包
  internal/  lint/tsconfig/vite/tailwind 配置
sdk/web/uba/  纯 TS SDK,tsc 构建
```

## 动手前默认动作

- 改业务页:优先看 `views/app/data-analysis/` 是否已有同类模型可复用,以及共享 `components/`。
- 调后端接口:用 `generated/api` 里已生成的类型化方法,不要手写 URL/类型。
- 实时数据:走 `transport/sse/`,不轮询。
- 加依赖:先查 catalog,内部包用 `workspace:*`。
- 提交前:`pnpm check:type` + `pnpm lint` 通过。

## 前后端契约

后端 proto 是唯一真相源;后端 `make api` 会把 TypeScript 调用代码生成进 `apps/admin/src/generated/api/`。前端断链时,去后端 proto 或生成插件排查,而非改生成物。

详见 `AGENTS.md`。
