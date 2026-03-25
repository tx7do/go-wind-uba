package data

import (
	"github.com/go-kratos/kratos/v2/registry"

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
const UseClickHouse bool = true
