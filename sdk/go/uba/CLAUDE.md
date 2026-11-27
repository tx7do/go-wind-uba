# CLAUDE.md — sdk/go/uba 速记

- Go 服务端 SDK，零依赖（标准库）；deviceId=hostname，platform=`server`。
- 契约真相源：`backend/api/protos/`。camelCase、body 鉴权、无 gzip、单批 <1MiB、超时 8s。
- `tenantId`/`serverTime`/`ip` 不发；数值标量只进 behavior oneof（金样锁定）。
- 重试：指数退避 1s×2^n×3；4xx(除429) 不重试不回队；可重试耗尽回队由定时器补发（cap 10000）。
- `MaxRetries` 0=默认3、负数=禁用；`FlushInterval` 负数=禁用定时器。
- 提交前：`go vet ./... && go test ./... -count=1 -race`。
