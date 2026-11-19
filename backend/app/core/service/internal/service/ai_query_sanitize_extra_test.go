package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

// ── sanitizeSQLFor 补充护栏:LIMIT 钳制与表名纠正 ──

// 无 LIMIT 的语句自动包一层 LIMIT(aiQueryRowLimit),防止全表扫爆内存。
func TestSanitizeSQL_AddsLimitWhenMissing(t *testing.T) {
	got, err := sanitizeSQLFor("SELECT event_name FROM events_fact", nil)
	require.NoError(t, err)
	assert.Contains(t, got, "LIMIT 100", "无 LIMIT 应自动补上限")
	assert.True(t, strings.HasPrefix(got, "SELECT * FROM ("), "应以子查询包裹原语句")
}

// 超上限的显式 LIMIT 必须被改写为上限值。
func TestSanitizeSQL_ClampsOversizedLimit(t *testing.T) {
	got, err := sanitizeSQLFor("SELECT event_name FROM events_fact LIMIT 999999", nil)
	require.NoError(t, err)
	assert.Contains(t, got, "LIMIT 100")
	assert.NotContains(t, got, "999999")

	// 大写 LIMIT 同样处理。
	got, err = sanitizeSQLFor("SELECT event_name FROM events_fact LIMIT 5000", nil)
	require.NoError(t, err)
	assert.Contains(t, got, "LIMIT 100")
}

// 合法范围内的 LIMIT 原样保留。
func TestSanitizeSQL_KeepsReasonableLimit(t *testing.T) {
	got, err := sanitizeSQLFor("SELECT event_name FROM events_fact LIMIT 10", nil)
	require.NoError(t, err)
	assert.Contains(t, got, "LIMIT 10")
	assert.False(t, strings.HasPrefix(got, "SELECT * FROM ("), "已有 LIMIT 不应再包子查询")
}

// WITH(CTE)语法合法,但 CTE 中间表名同样会撞表白名单 —— 当前实现按字面
// 白名单逐一校验 FROM/JOIN 的每个标识符,临时表别名(如 t)会被拒
// 【现状钉死,fail-closed 语义:模型想借 CTE 引入白名单外表同样走不通】。
func TestSanitizeSQL_AllowsWithCte(t *testing.T) {
	raw := "WITH t AS (SELECT event_name FROM events_fact) SELECT event_name FROM t LIMIT 10"
	_, err := sanitizeSQLFor(raw, nil)
	require.Error(t, err, "CTE 临时表名 t 不在白名单,当前实现直接拒绝")
	assert.Contains(t, err.Error(), "whitelist")
}

// 模型把白名单表写成复数时自动纠正为单数。
func TestSanitizeSQL_CorrectsPluralTableName(t *testing.T) {
	got, err := sanitizeSQLFor("SELECT event_name FROM events_facts LIMIT 10", nil)
	require.NoError(t, err)
	assert.Contains(t, got, "FROM events_fact", "复数表名应纠正为白名单拼写")
	assert.NotContains(t, got, "events_facts")
}

// 白名单纠正后仍不认识的表必须被拒。
func TestSanitizeSQL_RejectsUnknownTableEvenAfterCorrection(t *testing.T) {
	if _, err := sanitizeSQLFor("SELECT * FROM password_hashes LIMIT 10", nil); err == nil {
		t.Fatal("whitelist-external table must be rejected")
	}
}

// 空输出 / 非只读开头必须被拒。
func TestSanitizeSQL_RejectsNonSelect(t *testing.T) {
	cases := []string{
		"",              // 无 SQL
		"随便说点什么,没有 SQL", // 提取不到 SELECT
		"SHOW TABLES",   // 非 SELECT/WITH
		"INSERT INTO events_fact VALUES(1)",
		"( UPDATE events_fact SET a=1 )", // 括号包裹的写操作
	}
	for _, raw := range cases {
		if _, err := sanitizeSQLFor(raw, nil); err == nil {
			t.Fatalf("expected rejection for %q", raw)
		}
	}
}

// tenantIDPatternFor 容忍别名与空白(别名列、反引号附近的空格变体)。
func TestTenantIDPatternFor(t *testing.T) {
	pat := tenantIDPatternFor(7)
	assert.True(t, pat.MatchString("WHERE tenant_id = 7"))
	assert.True(t, pat.MatchString("WHERE tenant_id=7"))
	assert.True(t, pat.MatchString("WHERE  tenant_id   =   7"))
	assert.True(t, pat.MatchString("WHERE Tenant_ID = 7"), "大小写不敏感")
	assert.True(t, pat.MatchString("WHERE e.tenant_id = 7"), "别名前缀")
	assert.False(t, pat.MatchString("WHERE tenant_id = 8"))
	assert.False(t, pat.MatchString("WHERE tenant_id = 71"), "不得前缀误匹配")
}

// ── extractSQL / stripSQLFence ──

func TestExtractSQL(t *testing.T) {
	// 前导解释文字:从第一个 SELECT 起截取。
	got := extractSQL("好的,这是查询:\nSELECT 1 FROM events_fact LIMIT 5;\n以上。")
	assert.Equal(t, "SELECT 1 FROM events_fact LIMIT 5", got)

	// 分号截断(多语句只留首条)。
	got = extractSQL("SELECT 1; DROP TABLE x")
	assert.Equal(t, "SELECT 1", got)

	// 行注释剥离(注释截断后该行尾部空格保留,整体仅首尾 trim —— 现状)。
	got = extractSQL("SELECT 1 -- 注释\nFROM events_fact LIMIT 5")
	assert.Equal(t, "SELECT 1 \nFROM events_fact LIMIT 5", got)

	// 无 SQL 返回空串。
	assert.Equal(t, "", extractSQL("这段话没有查询"))
	assert.Equal(t, "", extractSQL(""))
}

func TestStripSQLFence(t *testing.T) {
	// ```sql 围栏。
	assert.Equal(t, "SELECT 1", stripSQLFence("```sql\nSELECT 1\n```"))
	// ``` 围栏(无语言标注)。
	assert.Equal(t, "SELECT 1", stripSQLFence("```\nSELECT 1\n```"))
	// 围栏未闭合:剥头不炸尾。
	assert.Equal(t, "SELECT 1", stripSQLFence("```sql\nSELECT 1"))
	// 无围栏原样返回(trim)。
	assert.Equal(t, "SELECT 1", stripSQLFence("  SELECT 1  "))
	// 大写 SQL 标注围栏。
	assert.Equal(t, "SELECT 1", stripSQLFence("```SQL\nSELECT 1\n```"))
}

// ── correctTableName ──

func TestCorrectTableName(t *testing.T) {
	// 候选序列为 table+s / fact_+table / 去尾 s / 去尾 s+es(依次尝试)。
	whitelist := map[string]string{"risk_events": "desc", "fact_events": "desc"}

	// 精确命中。
	got, ok := correctTableName("risk_events", whitelist)
	assert.True(t, ok)
	assert.Equal(t, "risk_events", got)

	// 少写复数:risk_event → risk_events(候选 table+s)。
	got, ok = correctTableName("risk_event", whitelist)
	assert.True(t, ok)
	assert.Equal(t, "risk_events", got)

	// 丢 fact_ 前缀:events → fact_events(候选 fact_+table)。
	got, ok = correctTableName("events", whitelist)
	assert.True(t, ok)
	assert.Equal(t, "fact_events", got)

	// 多写复数:events → 候选去尾 s 命中 fact_events? 否 —— 去尾得 "event",
	// 候选 "event"+"es" 不命中;最终拒绝【钉现状:候选是按序尝试,不做模糊匹配】。
	_, ok = correctTableName("eventss", whitelist)
	assert.False(t, ok)

	// 完全不认识 → 拒绝。
	_, ok = correctTableName("users", whitelist)
	assert.False(t, ok)
	_, ok = correctTableName("", whitelist)
	assert.False(t, ok)
}

// ── structpb → 问数响应行 ──
// (stringifyStructValue 的逐类型断言见 ai_prompt_convert_test.go,此处不重复。)

func TestStructpbRowsToStringRows(t *testing.T) {
	s1, _ := structpb.NewValue("2024-01-01")
	s2, _ := structpb.NewValue(42.0)
	rows := []*ubaV1.SqlResultRow{
		{Values: []*structpb.Value{s1, s2}},
	}
	got := structpbRowsToStringRows(rows)
	require.Len(t, got, 1)
	assert.Equal(t, []string{"2024-01-01", "42"}, got[0].Values)

	// 空输入 → 空切片(非 nil)。
	assert.NotNil(t, structpbRowsToStringRows(nil))
	assert.Empty(t, structpbRowsToStringRows(nil))
}

// ── 热点 benchmark:每条问数请求都会走两遍 ──

func BenchmarkSanitizeSQLFor(b *testing.B) {
	raw := "```sql\nSELECT event_name, COUNT(DISTINCT user_id) AS uv\nFROM events_fact\nWHERE tenant_id = 7 AND event_time >= NOW() - INTERVAL 7 DAY\nGROUP BY event_name ORDER BY uv DESC LIMIT 10000\n```"
	pat := tenantIDPatternFor(7)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := sanitizeSQLFor(raw, pat); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkExtractSQL(b *testing.B) {
	raw := "这是查询:\n```sql\nSELECT event_name FROM events_fact WHERE tenant_id = 7 LIMIT 100\n```\n说明文字。"
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if extractSQL(raw) == "" {
			b.Fatal("empty sql")
		}
	}
}
