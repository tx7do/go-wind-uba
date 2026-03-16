package schema

import _ "embed"

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
