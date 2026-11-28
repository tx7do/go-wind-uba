package ingestapp

import (
	"strings"
	"testing"

	"go-wind-uba/pkg/dorisinit"
)

// decisionPhrase 把 ensure 动作翻成面向操作者的"apply 将会做什么"句式;
// 未知动作原样返回(不吞掉未来新增的动作名)。
func TestDecisionPhrase(t *testing.T) {
	cases := map[string]string{
		string(dorisinit.ActionCreated):       "create it",
		string(dorisinit.ActionResumed):       "resume it",
		string(dorisinit.ActionAlreadyActive): "leave it alone",
		string(dorisinit.ActionNeedsHuman):    "leave it to a human — restarting a terminal job replays the topic",
		string(dorisinit.ActionFailed):        "fail on it",
		"undeclared":                          "leave it alone",
		"future-action":                       "future-action", // 未知动作透传
		"":                                    "",              // 空动作透传
	}
	for in, want := range cases {
		if got := decisionPhrase(in); got != want {
			t.Errorf("decisionPhrase(%q) = %q, want %q", in, got, want)
		}
	}
}

// displayJob 无库限定时回退默认库,为展示口径的兜底。
func TestDisplayJob(t *testing.T) {
	cases := []struct{ defaultDB, db, name, want string }{
		{"gw_uba", "", "job_a", "gw_uba.job_a"},           // 无库限定 → DSN 指向的库
		{"gw_uba", "other_db", "job_a", "other_db.job_a"}, // 显式库优先
		{"", "", "job_a", "job_a"},                        // 双空 → 裸任务名
		{"", "db", "job_a", "db.job_a"},
	}
	for _, c := range cases {
		if got := displayJob(c.defaultDB, c.db, c.name); got != c.want {
			t.Errorf("displayJob(%q,%q,%q) = %q, want %q", c.defaultDB, c.db, c.name, got, c.want)
		}
	}
}

// counters 从 Statistic JSON 里解出可读行数;字段缺失/损坏时尽力而为不报错。
func TestCounters(t *testing.T) {
	rows, _ := counters(newJob("gw_uba", "j", "RUNNING", `{"totalRows":100,"loadedRows":99,"errorRows":1}`))
	if rows != "loaded 99 of 100, 1 errors" {
		t.Errorf("rows = %q", rows)
	}

	// 损坏的 JSON:rows 为空串,不得 panic。
	rows, _ = counters(newJob("gw_uba", "j", "RUNNING", `not-json`))
	if rows != "" {
		t.Errorf("corrupted statistic should degrade to empty, got %q", rows)
	}

	// 无该列:同样空串。
	rows, _ = counters(newJob("gw_uba", "j", "RUNNING", ""))
	if rows != "" {
		t.Errorf("missing statistic should degrade to empty, got %q", rows)
	}
}

// orDash 空值展示为 "-",保持表格列对齐。
func TestOrDash(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "-"},
		{"   ", "-"},
		{"value", "value"},
		{"0", "0"}, // "0" 是有效值,不是空
	}
	for _, c := range cases {
		if got := orDash(c.in); got != c.want {
			t.Errorf("orDash(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// additiveWarning 提醒操作者哪些小节写 AGGREGATE 表(重复跑会累加)。
func TestAdditiveWarning(t *testing.T) {
	// 默认幂等小节:无警告。
	if got := additiveWarning(defaultETLSections); got != "" {
		t.Errorf("default sections should not warn, got %q", got)
	}
	// 包含 AGGREGATE 小节:警告必须点名该小节并给出幂等选项。
	got := additiveWarning([]string{"1", "3"})
	if !strings.Contains(got, "3") {
		t.Errorf("warning should name the additive section: %q", got)
	}
	if !strings.Contains(got, "--sections") {
		t.Errorf("warning should suggest --sections: %q", got)
	}
	// 多个小节:全部点名。
	got = additiveWarning([]string{"3", "4", "5"})
	for _, s := range []string{"3", "4", "5"} {
		if !strings.Contains(got, s) {
			t.Errorf("warning missing section %s: %q", s, got)
		}
	}
}

// sectionsOf 按步骤顺序去重收集小节号。
func TestSectionsOf(t *testing.T) {
	report := &dorisinit.ETLReport{
		Steps: []dorisinit.ETLStep{
			{Section: "1"},
			{Section: "1"},
			{Section: "2"},
			{Section: "2"},
			{Section: "1"}, // 回跳也各算一次
		},
	}
	got := sectionsOf(report)
	if strings.Join(got, ",") != "1,2,1" {
		t.Errorf("sectionsOf = %v, want [1 2 1]", got)
	}
}

// endpoint 的密码脱敏:任何打印形态都不得出现明文口令(安全口径)。
func TestSettingsEndpointMasksPasswordVariants(t *testing.T) {
	dsns := []string{
		"root:secret123@tcp(doris-fe:9030)/gw_uba",
		"user:p%40ss@tcp(127.0.0.1:9030)/db?charset=utf8mb4",
	}
	for _, dsn := range dsns {
		s := &settings{dsn: dsn, configPath: "configs"}
		if strings.Contains(s.endpoint(), "secret") {
			t.Errorf("endpoint leaks password: %q", s.endpoint())
		}
	}
	// describe 也不得泄漏。
	s := &settings{dsn: "root:secret123@tcp(doris-fe:9030)/gw_uba", configPath: "configs"}
	if strings.Contains(s.describe(), "secret123") {
		t.Errorf("describe leaks password: %q", s.describe())
	}
	if !strings.Contains(s.describe(), "gw_uba") {
		t.Errorf("describe should name the database: %q", s.describe())
	}
}

// database 从 DSN 解析库名;坏 DSN 优雅返回空串。
func TestSettingsDatabase(t *testing.T) {
	s := &settings{dsn: "root:x@tcp(h:9030)/gw_uba"}
	if got := s.database(); got != "gw_uba" {
		t.Errorf("database = %q", got)
	}
	s = &settings{dsn: "not-a-dsn"}
	if got := s.database(); got != "" {
		t.Errorf("bad dsn should give empty database, got %q", got)
	}
}

// BenchmarkJobViews status 命令的声明-上报合并(几十个任务时逐秒轮询路径)。
func BenchmarkJobViews(b *testing.B) {
	declared := make([]dorisinit.RoutineLoadJob, 0, 50)
	reported := make([]dorisinit.Job, 0, 50)
	for i := 0; i < 50; i++ {
		name := "job_events_to_fact"
		declared = append(declared, dorisinit.RoutineLoadJob{Database: "gw_uba", Name: name, Table: "events_fact"})
		reported = append(reported, newJob("gw_uba", name, "RUNNING", `{"totalRows":1000,"loadedRows":990,"errorRows":10}`))
	}
	results := dorisinit.Decide(declared, reported, dorisinit.EnsurePolicy{Database: "gw_uba", ResumePaused: true, DryRun: true})
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		jobViews(results, reported, "gw_uba")
	}
}

// BenchmarkAbbrev 诊断列折叠(逐行逐格渲染)。
func BenchmarkAbbrev(b *testing.B) {
	text := strings.Repeat("word ", 200)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		abbrev(text, 8)
	}
}
