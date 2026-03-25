package client

import (
	dorisCrud "github.com/tx7do/go-crud/doris"

	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"github.com/tx7do/kratos-bootstrap/database/doris"

	"go-wind-uba/app/core/service/internal/data"
)

func NewDorisClient(ctx *bootstrap.Context) (*dorisCrud.Client, func(), error) {
	if data.UseClickHouse {
		return nil, func() {}, nil
	}

	cfg := ctx.GetConfig()
	if cfg == nil {
		return nil, func() {}, nil
	}

	cli, err := doris.NewClient(ctx.GetLogger(), cfg)
	if err != nil {
		return nil, func() {}, err
	}

	return cli, func() {
	}, nil
}
