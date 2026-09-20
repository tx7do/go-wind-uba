// Package doris holds the Doris provisioning scripts as embedded assets, so the ingest CLI
// can run inside the slim service image (which ships only the binary and ./configs).
package doris

import (
	_ "embed"

	"go-wind-uba/pkg/dorisinit"
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

// InitScripts returns the scripts that provision gw_uba, in apply order.
//
// 05_views.sql is intentionally left out. Its materialized views refresh on every commit, so
// they would tax each Routine Load publish, and the aggregate tables and views they feed have
// no reader in this repository yet: the analytics repos query events_fact / sessions_fact
// directly. Render it with `uba-ingest render --script 05_views.sql` when that changes.
//
// 04_indexes.sql currently holds only comments; it stays in the list so that adding an index
// needs no code change.
func InitScripts() []dorisinit.Script {
	return []dorisinit.Script{
		{Name: "1_base_tables.sql", Bytes: BaseTablesData},
		{Name: "02_kafka_tables.sql", Bytes: KafkaTablesData},
		{Name: "03_aggregate_tables.sql", Bytes: AggregateTablesData},
		{Name: "04_indexes.sql", Bytes: IndexesData},
	}
}

// EtlScript returns the daily aggregation script.
func EtlScript() dorisinit.Script {
	return dorisinit.Script{Name: "06_etl.sql", Bytes: EtlData}
}

// AllScripts returns every embedded script by name, for the render subcommand.
func AllScripts() []dorisinit.Script {
	return append(InitScripts(),
		dorisinit.Script{Name: "05_views.sql", Bytes: ViewsData},
		EtlScript(),
	)
}
