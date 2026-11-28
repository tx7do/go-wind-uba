// Command uba-ingest provisions and operates the OLAP side of the UBA event pipeline.
// 实现承载在 go-wind-uba/pkg/ingestapp(可导入包),本文件只是开源构建的入口壳;
// 企业版入口(go-wind-uba-enterprise/tools/uba-ingest)复用同一命令面并前置 license 门闩。
//
// Usage:
//
//	uba-ingest -c /app/configs apply --dry-run
//	uba-ingest -c /app/configs etl --loop --at 02:00
//	uba-ingest -c /app/configs retention --policy retention.json   # 企业能力
package main

import (
	"os"

	ingestapp "go-wind-uba/pkg/ingestapp"
)

func main() {
	os.Exit(ingestapp.Run(os.Args[1:]))
}
