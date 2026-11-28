package data

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	conf "github.com/tx7do/kratos-bootstrap/api/gen/go/conf/v1"
	"github.com/tx7do/kratos-bootstrap/bootstrap"
)

func newEngineTestCtx(engine string) *bootstrap.Context {
	return bootstrap.NewContextWithParam(
		context.Background(),
		&conf.AppInfo{}, &conf.Bootstrap{
			Data: &conf.Data{OlapEngine: &engine},
		},
		log.DefaultLogger,
	)
}

func TestUseEngineFromCtx(t *testing.T) {
	t.Cleanup(func() {
		UseClickHouse = false
		useEngineApplied = false
	})

	// clickhouse:置位
	UseClickHouse = false
	useEngineApplied = false
	UseEngineFromCtx(newEngineTestCtx("clickhouse"))
	if !UseClickHouse {
		t.Fatal("UseClickHouse = false, want true")
	}

	// 幂等:二次调用不改变已生效的开关
	UseEngineFromCtx(newEngineTestCtx("doris"))
	if !UseClickHouse {
		t.Fatal("second apply changed switch, want idempotent")
	}

	// 重置后 doris:复位
	useEngineApplied = false
	UseClickHouse = true
	UseEngineFromCtx(newEngineTestCtx("doris"))
	if UseClickHouse {
		t.Fatal("UseClickHouse = true, want false after doris")
	}

	// 缺省(空值):回退 Doris
	useEngineApplied = false
	UseClickHouse = true
	UseEngineFromCtx(newEngineTestCtx(""))
	if UseClickHouse {
		t.Fatal("UseClickHouse = true, want false on empty config")
	}
}
