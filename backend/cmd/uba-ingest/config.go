package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"

	mysql "github.com/go-sql-driver/mysql"
	conf "github.com/tx7do/kratos-bootstrap/api/gen/go/conf/v1"
	bConfig "github.com/tx7do/kratos-bootstrap/config"

	"go-wind-uba/pkg/dorisinit"
)

// settings is what a subcommand works with: where to connect, and where the template
// parameters come from. Both are resolved from the service configs or the environment, so a
// credential never has to appear on a command line.
type settings struct {
	configPath string
	dsn        string

	// kafkaOverride comes from --kafka-brokers and beats both the environment and the
	// configs: which address the Doris BE should reach is a deployment fact an operator may
	// know better than the config baked for another network.
	kafkaOverride string

	// cfg may be nil, which the generated getters treat as "nothing configured". The broker
	// list is resolved lazily so a script that declares no job does not fail because kafka
	// is unconfigured.
	cfg *conf.Bootstrap
}

// loadSettings reads the configs and applies the environment overrides. It tolerates a DSN
// that is not set, because render connects to nothing; connect is what insists on one.
func loadSettings(path, kafkaOverride string) (*settings, error) {
	s := &settings{configPath: path, kafkaOverride: kafkaOverride}
	s.dsn = os.Getenv(envDorisDSN)

	if _, err := os.Stat(path); err != nil {
		return s, fmt.Errorf("config path %s: %w", path, err)
	}
	if err := bConfig.LoadBootstrapConfig(path); err != nil {
		return s, fmt.Errorf("load configs from %s: %w", path, err)
	}
	s.cfg = bConfig.GetBootstrapConfig()

	if s.dsn == "" {
		s.dsn = s.cfg.GetData().GetDoris().GetDsn()
	}
	if s.dsn == "" {
		return s, nil
	}
	if _, err := mysql.ParseDSN(s.dsn); err != nil {
		return s, fmt.Errorf("data.doris.dsn is not a valid MySQL-protocol DSN (user:pass@tcp(host:9030)/gw_uba): %w", err)
	}
	return s, nil
}

// database is the name the DSN points at. Doris scopes SHOW ROUTINE LOAD to it and reports it
// as DbName, which is how an unqualified job declaration gets resolved to a real job.
func (s *settings) database() string {
	dsn, err := mysql.ParseDSN(s.dsn)
	if err != nil {
		return ""
	}
	return dsn.DBName
}

// brokerList renders the kafka_broker_list property Doris connects to.
func (s *settings) brokerList() (string, error) {
	endpoints := strings.Split(s.kafkaOverride, ",")
	if s.kafkaOverride == "" {
		if v := os.Getenv(envKafkaBrokers); v != "" {
			endpoints = strings.Split(v, ",")
		} else {
			endpoints = s.cfg.GetData().GetKafka().GetEndpoints()
		}
	}
	list, err := dorisinit.BrokerList(endpoints)
	if err != nil {
		return "", fmt.Errorf("no usable kafka broker list (%v): set data.kafka.endpoints in %s, or %s, or --kafka-brokers",
			err, s.configPath, envKafkaBrokers)
	}
	return list, nil
}

// buildParams renders the template parameters the given scripts reference, plus the run date
// when one is supplied. A script that references neither is applied with an empty parameter
// set, which is what makes "apply --script 1_base_tables.sql" work before kafka is wired up.
func (s *settings) buildParams(scripts []dorisinit.Script, runDate string) (dorisinit.Params, error) {
	params := dorisinit.Params{}

	if references(scripts, "KafkaBrokerList") {
		list, err := s.brokerList()
		if err != nil {
			return nil, err
		}
		params["KafkaBrokerList"] = list
	}
	if references(scripts, "RunDate") {
		if runDate == "" {
			return nil, fmt.Errorf("these scripts are parameterised by run date; pass --date YYYY-MM-DD")
		}
		if err := dorisinit.CheckRunDate(runDate); err != nil {
			return nil, err
		}
		params["RunDate"] = runDate
	}
	return params, nil
}

func references(scripts []dorisinit.Script, param string) bool {
	for _, sc := range scripts {
		if strings.Contains(string(sc.Bytes), param) {
			return true
		}
	}
	return false
}

// connect opens the pool the subcommand works through. dorisinit takes one dedicated
// connection per unit of work, because "USE gw_uba" and the session SETs in the ETL script
// are per-connection in Doris.
func (s *settings) connect(ctx context.Context) (dorisinit.Source, func(), error) {
	if s.dsn == "" {
		return nil, func() {}, fmt.Errorf("no Doris DSN: set data.doris.dsn in %s (or the environment variable %s)",
			s.configPath, envDorisDSN)
	}
	db, err := sql.Open("mysql", s.dsn)
	if err != nil {
		return nil, func() {}, fmt.Errorf("open doris (%s): %w", s.endpoint(), err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, func() {}, fmt.Errorf("ping doris (%s): %w", s.endpoint(), err)
	}
	return dorisinit.WrapDB(db), func() { _ = db.Close() }, nil
}

// endpoint names the target without its password, which is the only form this tool prints.
func (s *settings) endpoint() string {
	if s.dsn == "" {
		return "no dsn configured"
	}
	cfg, err := mysql.ParseDSN(s.dsn)
	if err != nil {
		return "<unparsable dsn>"
	}
	cfg.Passwd = ""
	return cfg.FormatDSN()
}

// describe is the one line that says where a run is pointed.
func (s *settings) describe() string {
	if db := s.database(); db != "" {
		return fmt.Sprintf("%s · database %s · configs %s", s.endpoint(), db, s.configPath)
	}
	return fmt.Sprintf("%s · configs %s", s.endpoint(), s.configPath)
}
