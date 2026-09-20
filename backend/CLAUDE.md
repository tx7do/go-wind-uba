# CLAUDE.md — go-wind-uba backend

本文件供 Claude / Claude Code 在 `backend/` 目录工作时使用。
权威与完整指南见同目录 `AGENTS.md`;两者保持同步,内容等价。
下面是 Claude 最该先记住的要点,详细约定请读 `AGENTS.md`。

## 你在哪里

`go-wind-uba` 的后端:Go 1.25 + Kratos v2 + Ent + Wire 的多服务单体仓库。
三个服务:`app/admin`、`app/collector`(SDK 埋点采集)、`app/core`(核心 + 数据分析)。
业务数据进 PostgreSQL,行为/分析数据进 Apache Doris(可选 ClickHouse)。

## 三条铁律

1. **生成物不可手改。** 这些全是产物,改源头再重新生成:
   - `api/gen/`(来自 `api/protos/`,用 `make api`)
   - Ent 生成代码(来自 schema,用 `make ent`)
   - `*/wire_gen.go`(来自 `wire.go`,用 `make wire`)—— 且需提交

2. **改任何契约先想“重新生成链”。** 改 proto → `make api`;改 ent schema → `make ent`;改 ProviderSet → `make wire`。漏一步就会让编译/前端断链。

3. **Doris 分析查询易踩坑。** 核心分析逻辑在 `app/core/service/internal/data/doris/`。
   - `time.Time` 列扫描要先转字符串/用指针,避免历史已修过的 scan 报错复发。
   - 时间分桶注意时区一致性。
   - OLAP 只做分析,事务归 PostgreSQL。

## 目录速记

```
api/protos/   契约真相源(按业务域)
api/gen/      生成物(禁改)
app/<svc>/service/internal/{data,service,server}  DDD 三层
cmd/uba-ingest/  入仓运维 CLI(schema / Routine Load / ETL 调度 / 健康度)
sql/{postgresql,doris,clickhouse}/  DDL + 初始数据
pkg/          跨服务基础库(入仓的纯逻辑在 pkg/dorisinit)
Makefile      api/ent/wire/build/lint/test/ingest
```

## 数据入仓(别写 Go 消费者)

kafka → Doris 的搬运由 **Doris Routine Load** 负责(FE 管调度与位点),Go 侧只做装配、调度、
观测,入口 `uba-ingest`:`apply`(幂等建表 + 确保任务在跑)/ `status`(健康度,可当
healthcheck)/ `etl [--loop --at 02:00]`(每日回算)/ `render`(只渲染不连库)。

- DSN、broker 只从 configs 或 `UBA_DORIS_DSN` / `UBA_KAFKA_BROKERS` 来,**不要加接收密钥的命令行参数**。
- 退出码 `2` = 需要人工(任务处于终态,自动重建会重放整个 topic)。
- `etl` 默认只跑 `06_etl.sql` 的 §1/§2(UNIQUE KEY 表,可重跑);§3~§5 写 AGGREGATE 表,
  同一天算两次是**累加**,要显式 `--sections` 且保证一天只算一次。
- `sql/doris/*.sql` 不许带 BOM、统一 LF(有守门测试 `sql/doris/assets_test.go`)。

## 动手前默认动作

- 涉及 API:先看 `api/protos/<domain>/`,改完 `make api`。
- 涉及模型:先看 ent schema 与 `sql/`,改完 `make ent` 并同步 DDL。
- 涉及依赖:`wire.go` 改完 `make wire`。
- 提交前:`go build ./...` + `go vet ./...`;生成物随源头一起提交。

## 错误处理约定

用 Kratos errors(由 `protoc-gen-go-errors` 从 proto 生成错误码)。新增错误码在 proto 里定义,前端依赖这些码做 i18n,不要在代码里返回裸字符串错误给前端。

详见 `AGENTS.md`。
