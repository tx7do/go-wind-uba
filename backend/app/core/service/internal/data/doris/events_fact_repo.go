package doris

import (
	"github.com/go-kratos/kratos/v2/log"
	dorisCrud "github.com/tx7do/go-crud/doris"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
)

type EventsFactRepo struct {
	db  *dorisCrud.Client
	log *log.Helper
}

func NewEventsFactRepo(
	ctx *bootstrap.Context,
	db *dorisCrud.Client,
) *EventsFactRepo {
	return &EventsFactRepo{
		log: ctx.NewLoggerHelper("events-fact/doris/repo/core-service"),
		db:  db,
	}
}
