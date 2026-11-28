package retention

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writePolicy(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func TestLoadFromFile(t *testing.T) {
	if policy, err := LoadFromFile(filepath.Join(t.TempDir(), "missing.json")); policy != nil || err != nil {
		t.Fatalf("missing file = (%v, %v), want (nil, nil)", policy, err)
	}

	path := writePolicy(t, `[
		{"table":"events_fact","days":365,"time_column":"event_date"},
		{"table":"sessions_fact","days":90}
	]`)
	policy, err := LoadFromFile(path)
	if err != nil {
		t.Fatalf("LoadFromFile() = %v", err)
	}
	if len(policy) != 2 || policy[0].Table != "events_fact" || policy[0].Days != 365 {
		t.Fatalf("policy = %+v", policy)
	}

	bad := writePolicy(t, `[{"table":"events_fact","days":0}]`)
	if _, err := LoadFromFile(bad); err == nil || !strings.Contains(err.Error(), "days") {
		t.Fatalf("zero days = %v, want error", err)
	}
	bad = writePolicy(t, `[{"table":"a","days":10},{"table":"a","days":20}]`)
	if _, err := LoadFromFile(bad); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate table = %v, want error", err)
	}
	bad = writePolicy(t, `[{"days":10}]`)
	if _, err := LoadFromFile(bad); err == nil {
		t.Fatal("missing table = nil, want error")
	}
}

func TestDorisDDL(t *testing.T) {
	got := DorisSetDynamicPartitionStart("gw_uba.events_fact", 365)
	want := `ALTER TABLE gw_uba.events_fact SET ("dynamic_partition.start" = "-365")`
	if got != want {
		t.Fatalf("DDL = %s", got)
	}
}

func TestClickHouseDDL(t *testing.T) {
	got := ClickHouseModifyTTL("gw_uba.events_fact", "event_date", 365)
	want := "ALTER TABLE gw_uba.events_fact MODIFY TTL event_date + INTERVAL 365 DAY"
	if got != want {
		t.Fatalf("DDL = %s", got)
	}
}

func TestPolicyJSONRoundTrip(t *testing.T) {
	b, _ := json.Marshal(Rule{Table: "events_fact", Days: 90})
	if !strings.Contains(string(b), `"days":90`) || !strings.Contains(string(b), `"table":"events_fact"`) {
		t.Fatalf("tags broken: %s", b)
	}
}
