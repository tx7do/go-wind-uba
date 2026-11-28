// Package clickhouse holds the ClickHouse provisioning scripts as embedded assets, so the
// ingest CLI can run inside the slim service image (which ships only the binary and ./configs).
package clickhouse

import (
	_ "embed"

	"go-wind-uba/pkg/clickhouseinit"
)

//go:embed 1_base_tables.sql
var BaseTablesData []byte

//go:embed 02_kafka_tables.sql
var KafkaTablesData []byte

//go:embed 03_aggregate_tables.sql
var AggregateTablesData []byte

//go:embed 04_indexes.sql
var IndexesData []byte

//go:embed 05_views.sql
var ViewsData []byte

//go:embed 06_etl.sql
var EtlData []byte

//go:embed 07_attribution.sql
var AttributionData []byte

// InitScripts returns the scripts that provision gw_uba, in apply order.
//
// Unlike the Doris side, 05_views.sql is mandatory here: its materialized views are the only
// thing that builds sessions_fact / users_dim / path_features from events_fact (Doris builds
// them with the nightly ETL instead), and skipping them leaves those tables permanently empty.
func InitScripts() []clickhouseinit.Script {
	return []clickhouseinit.Script{
		{Name: "1_base_tables.sql", Bytes: BaseTablesData},
		{Name: "02_kafka_tables.sql", Bytes: KafkaTablesData},
		{Name: "03_aggregate_tables.sql", Bytes: AggregateTablesData},
		{Name: "04_indexes.sql", Bytes: IndexesData},
		{Name: "05_views.sql", Bytes: ViewsData},
	}
}

// EtlScript returns the daily derived-table recomputation script
// (sessions_fact / users_dim full rebuild + OPTIMIZE FINAL), the ClickHouse
// counterpart of the Doris 06_etl.sql §1/§2.
func EtlScript() clickhouseinit.Script {
	return clickhouseinit.Script{Name: "06_etl.sql", Bytes: EtlData}
}
