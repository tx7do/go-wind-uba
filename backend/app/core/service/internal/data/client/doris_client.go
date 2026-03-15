package client

import (
	dorisCrud "github.com/tx7do/go-crud/doris"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
	"github.com/tx7do/kratos-bootstrap/database/doris"
)

func NewDorisClient(ctx *bootstrap.Context) (*dorisCrud.Client, func(), error) {
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
