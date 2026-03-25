package client

import (
	clickhouseCrud "github.com/tx7do/go-crud/clickhouse"

	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"github.com/tx7do/kratos-bootstrap/database/clickhouse"

	"go-wind-uba/app/core/service/internal/data"
)

func NewClickHouseClient(ctx *bootstrap.Context) (*clickhouseCrud.Client, func(), error) {
	if !data.UseClickHouse {
		return nil, func() {}, nil
	}

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
