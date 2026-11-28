package data

import (
	"github.com/go-kratos/kratos/v2/registry"
	"strings"

	"github.com/tx7do/go-utils/password"

	"github.com/tx7do/kratos-bootstrap/bootstrap"
	bRegistry "github.com/tx7do/kratos-bootstrap/registry"
)

// NewDiscovery 创建服务发现客户端
func NewDiscovery(ctx *bootstrap.Context) registry.Discovery {
	cfg := ctx.GetConfig()
	if cfg == nil {
		return nil
	}

	ret, err := bRegistry.NewDiscovery(cfg.Registry)
	if err != nil {
		return nil
	}
	return ret
}

func NewPasswordCrypto() password.Crypto {
	crypto, err := password.CreateCrypto("bcrypt")
	if err != nil {
		panic(err)
	}
	return crypto
}

// UseClickHouse 是否使用ClickHouse作为数据存储，否则使用Doris。
// 由部署配置 data.olapEngine（clickhouse | doris，缺省 doris）决定，
// 装配期经 UseEngineFromCtx 设置（NewData/客户端构造时应用）；之后不再变更。
var UseClickHouse = false

// useEngineApplied 防重复应用（幂等）。
var useEngineApplied bool

// UseEngineFromCtx 从框架配置读取 OLAP 引擎并设置路由开关（幂等，
// NewData 与各 OLAP 客户端构造都可安全调用）。配置缺省/非法值回退 Doris。
func UseEngineFromCtx(ctx *bootstrap.Context) {
	if ctx == nil || useEngineApplied {
		return
	}
	cfg := ctx.GetConfig()
	if cfg == nil {
		return
	}
	useEngineApplied = true
	UseClickHouse = strings.EqualFold(strings.TrimSpace(cfg.GetData().GetOlapEngine()), "clickhouse")
}
