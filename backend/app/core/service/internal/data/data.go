package data

import (
	"github.com/go-kratos/kratos/v2/registry"
	"go-wind-uba/pkg/olapcfg"

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
// 由部署配置决定：configs/olap.yaml 的 olap.engine，或环境变量 UBA_OLAP_ENGINE
// （后者优先）；进程启动时一次性装载，默认 Doris。
var UseClickHouse = olapcfg.UseClickHouse()
