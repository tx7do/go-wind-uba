package client

import (
	clickhouseCrud "github.com/tx7do/go-crud/clickhouse"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"github.com/tx7do/kratos-bootstrap/database/clickhouse"
)

func NewClickHouseClient(ctx *bootstrap.Context) (*clickhouseCrud.Client, func(), error) {
	cfg := ctx.GetConfig()
	if cfg == nil {
		return nil, func() {}, nil
	}

	cli, err := clickhouse.NewClient(ctx.GetLogger(), cfg)
	if err != nil {
		return nil, func() {}, err
	}

	return cli, func() {
	}, nil
}
