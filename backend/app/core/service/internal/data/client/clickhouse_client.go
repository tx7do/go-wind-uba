package client

import (
	"errors"

	clickhouseV2 "github.com/ClickHouse/clickhouse-go/v2"
	clickhouseCrud "github.com/tx7do/go-crud/clickhouse"

	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"github.com/tx7do/kratos-bootstrap/database/clickhouse"

	conf "github.com/tx7do/kratos-bootstrap/api/gen/go/conf/v1"

	"go-wind-uba/app/core/service/internal/data"
)

// NewClickHouseClient 返回 OLAP 仓储用的 crud 客户端，以及一条供自助 SQL 使用的
// 独立原生连接。crud 封装的 Select 只能扫进结构体，而自助查询的列是动态的，
// 需要 clickhouse-go 的原始行集（Columns + Scan）。
func NewClickHouseClient(ctx *bootstrap.Context) (*clickhouseCrud.Client, clickhouseV2.Conn, func(), error) {
	data.UseEngineFromCtx(ctx)

	if !data.UseClickHouse {
		return nil, nil, func() {}, nil
	}

	cfg := ctx.GetConfig()
	if cfg == nil {
		return nil, nil, func() {}, nil
	}

	cli, err := clickhouse.NewClient(ctx.GetLogger(), cfg)
	if err != nil {
		return nil, nil, func() {}, err
	}

	// adhoc 连接拿不到 crud client 的私有 options，按同一份配置再开一条。
	// 失败随主流程一并失败：连不上大概率是配置错误，启动期就该暴露。
	adhoc, adhocErr := openAdhocConn(cfg)

	return cli, adhoc, func() {
		if adhoc != nil {
			_ = adhoc.Close()
		}
	}, adhocErr
}

// openAdhocConn 按配置另开一条原生连接：优先 DSN，否则 addresses + 认证信息。
func openAdhocConn(cfg *conf.Bootstrap) (clickhouseV2.Conn, error) {
	ch := cfg.GetData().GetClickhouse()
	if ch == nil {
		return nil, errors.New("clickhouse config is nil")
	}

	if ch.GetDsn() != "" {
		opts, err := clickhouseV2.ParseDSN(ch.GetDsn())
		if err != nil {
			return nil, err
		}
		return clickhouseV2.Open(opts)
	}

	return clickhouseV2.Open(&clickhouseV2.Options{
		Addr: ch.GetAddresses(),
		Auth: clickhouseV2.Auth{
			Database: ch.GetDatabase(),
			Username: ch.GetUsername(),
			Password: ch.GetPassword(),
		},
	})
}
