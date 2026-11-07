# 贡献指南

感谢你对 GoWind UBA 的关注！本文档说明如何向本项目提交代码、文档与问题反馈。在开始之前，请先阅读 [README.md](./README.md) 了解项目定位与技术栈。

## 行为准则

参与本项目即代表你同意在讨论、Issue 与 PR 中保持友善与专业。

## 快速上手

1. **Fork 并克隆仓库**

   ```bash
   git clone https://github.com/<your-name>/go-wind-uba.git
   cd go-wind-uba
   git remote add upstream https://github.com/tx7do/go-wind-uba.git
   ```

2. **搭建开发环境**

   - 中间件依赖：Docker Compose 启动 PostgreSQL / Redis / Kafka / etcd / Doris / SeaweedFS（S3 兼容对象存储，容器服务名沿用 minio；见 `backend/docker-compose.libs.yaml`）
   - 后端：Go 1.25+，参考 [backend/AGENTS.md](./backend/AGENTS.md) 了解服务结构与代码生成
   - 前端：Node 18+ / pnpm，参考 [frontend/AGENTS.md](./frontend/AGENTS.md)
   - 根目录 [AGENTS.md](./AGENTS.md) 有全局数据流与模块速记

3. **初始化 Doris 表与 Routine Load**

   ```bash
   cd backend
   make build_only
   ./bin/uba-ingest -c app/core/service/configs apply --wait 5m
   ```

4. **启动服务**

   ```bash
   # 后端三服务（各终端或后台运行）
   cd backend
   go run ./app/core/service/cmd/server -conf app/core/service/configs
   go run ./app/admin/service/cmd/server -conf app/admin/service/configs
   go run ./app/collector/service/cmd/server -conf app/collector/service/configs

   # 前端 dev server
   cd frontend/admin && pnpm install && pnpm dev:antd
   ```

   > Windows 本地直跑时注意：如果系统代理（如 Clash）劫持了 localhost 请求，请在启动前设置 `NO_PROXY=localhost,127.0.0.1,<本机IP>`。

5. **创建分支**

   ```bash
   git checkout main
   git pull upstream main
   git checkout -b feat/your-feature
   ```

## 开发约定

### 代码生成

本项目大量依赖代码生成，**请不要手工编辑生成产物**：

| 产物 | 生成命令 | 说明 |
|------|---------|------|
| Go API / HTTP / gRPC / 校验代码 | `make api`（在 `backend/` 下） | 由 `api/protos/` 下的 proto 生成至 `api/gen/go/` |
| OpenAPI 文档 | `make openapi` | 产物写到各服务的 `assets/openapi.yaml` |
| TypeScript HTTP 客户端 | `make ts` | 生成到 `frontend/admin/apps/admin/src/api/generated/` |
| Ent ORM 代码 | `make ent` | 由 `app/core/service/internal/data/ent/schema/` 下的 schema 生成 |
| 新模块接入装配 | `make register ENTITY=xxx` | 把仓储/服务构造行与路由注册注入手写的 `cmd/server/wiring.go` 和 `internal/server/grpc_server.go`（靠 `// register:*` 锚点行定位） |

修改 `.proto` 或 `ent/schema` 后请重新生成并**连同产物一并提交**。

> **注意**：`make api` 里 buf 模板引用了 `buf.go.tag.gen.yaml`（旧版 buf 自动补 `.yaml` 后缀，新版不会）。如果你遇到 `buf.go.tag.gen` 找不到的报错，确认 Makefile 已改为 `buf.go.tag.gen.yaml`。
>
> `make register` 只覆盖 CRUD 生成器的标准形态。依赖更多的模块（如需要多个 repo、后台引擎、跨服务调用）请手工补 wiring 行。

### Proto 契约

- 契约单一真相源在 `backend/api/protos/`——所有 API 先改 proto 再 `make api` 生成。
- 领域 proto 放独立包（如 `notification/service/v1/`），admin BFF 壳放 `admin/service/v1/i_*.proto` 只加 HTTP 注解。
- 输入校验用 protoc-gen-validate 的 `(validate.rules)` 声明，不要在 service 层手写判空。
- 错误码用 protoc-gen-go-errors 的错误枚举声明（配 `errors/errors.proto`），生成 `Error<Prefix><Reason>()` 构造函数。

### 代码规范

- **Go**：遵循 `golangci-lint`（配置在 `backend/.golangci.yml`），提交前跑 `go vet ./...` 和 `go build ./...`。
- **前端**：`pnpm check:type` 必过（vue-tsc 全量类型检查），lint 用 `pnpm lint`。
- **命名与风格**：新代码应与所在文件既有风格保持一致（缩进、注释密度、命名习惯）。
- **多租户**：所有含租户字段的 schema 必须挂 `TenantMutationGuardPolicy`，新后台引擎用 `appViewer.NewSystemViewerContext(ctx)` 绕过隐私层。
- **文档**：README 三语同步——改功能清单后请按根 [AGENTS.md](./AGENTS.md) 的 checklist 同步中文/英文/日文三份。

### 提交规范

本项目采用 [Conventional Commits](https://www.conventionalcommits.org/zh-hans/)，提交信息格式：

```
<type>(<scope>): <subject>
```

| type | 用途 |
|------|------|
| `feat` | 新功能 |
| `fix` | 缺陷修复 |
| `refactor` | 重构（不改变外部行为） |
| `perf` | 性能优化 |
| `style` | 代码格式（不影响逻辑） |
| `docs` | 文档 |
| `test` | 补充测试 |
| `build` | 构建系统、依赖 |
| `chore` | 杂项、脚手架 |
| `ci` | CI 配置 |

- `scope` 可选，对应受影响的模块（如 `collect`、`notification`、`loop`、`experiment`）
- `subject` 使用中文或英文均可，简明描述，结尾不加句号
- **示例**：`feat(campaign): 营销触达——分群×文案×站内信的群发执行层`

大改动请添加正文说明动机与影响范围，必要时附 Issue 链接。

## 提交 Pull Request

1. 确保本地通过后端 `go build ./...` + `go vet ./...`，以及前端 `pnpm check:type`
2. 若改动涉及 proto / schema，确认已重新生成并提交产物：`make api && make openapi && make ent`（`make ts` 只在前端在范围内时跑）
3. PR 标题遵循上述提交规范
4. PR 描述请写清：类型、动机、改动内容、影响范围、自测结果
5. 一个 PR 只解决一个问题，便于评审与回退

### 评审标准

- 是否改动生成产物、是否漏跑生成
- 是否破坏多租户隔离（跨租户数据访问需带租户过滤）
- 是否引入新的硬编码密钥 / 凭据（应走配置或环境变量）
- 是否补齐了相应测试
- 文档是否同步更新

## 反馈问题

- 缺陷请开 Issue 并附复现步骤与环境信息
- 新功能建议请开功能请求 Issue
- 安全漏洞请按 [SECURITY.md](./SECURITY.md) 流程私下上报，勿直接开公开 Issue

## 致谢

你的每一次贡献——无论代码、文档还是问题反馈——都让 GoWind UBA 更好。感谢你的参与。
