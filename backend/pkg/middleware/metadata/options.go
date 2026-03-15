package metadata

import "github.com/go-kratos/kratos/v2/log"

type options struct {
	log *log.Helper

	extractMetadataFromServer bool
}

type Option func(*options)

// WithLogger 设置日志记录器
func WithLogger(logger log.Logger) Option {
	return func(o *options) {
		o.log = log.NewHelper(logger)
	}
}

// WithExtractMetadataFromServer 设置是否从服务器上下文中提取元数据
func WithExtractMetadataFromServer(extract bool) Option {
	return func(o *options) {
		o.extractMetadataFromServer = extract
	}
}
