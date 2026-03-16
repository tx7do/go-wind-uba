package clickhouse

import (
	"github.com/go-kratos/kratos/v2/log"
	clickhouseCrud "github.com/tx7do/go-crud/clickhouse"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
)

type EventsFactRepo struct {
	db  *clickhouseCrud.Client
	log *log.Helper
}

func NewEventsFactRepo(
	ctx *bootstrap.Context,
	db *clickhouseCrud.Client,
) *EventsFactRepo {
	return &EventsFactRepo{
		log: ctx.NewLoggerHelper("events-fact/ck/repo/core-service"),
		db:  db,
	}
}
