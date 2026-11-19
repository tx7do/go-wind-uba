package doris

import (
	"reflect"
	"testing"
	"time"

	ubaV1 "go-wind-uba/api/gen/go/uba/service/v1"
)

// 七天对应的毫秒数（normTimeRange 的默认回看窗口）。
const sevenDayMs = int64(7*24*time.Hour) / int64(time.Millisecond)

// mkPts 按 (时间戳, 值) 对构造时间序列点，避免用例间共享切片。
func mkPts(pairs ...[2]float64) []*ubaV1.TimeSeriesPoint {
	pts := make([]*ubaV1.TimeSeriesPoint, 0, len(pairs))
	for _, p := range pairs {
		pts = append(pts, &ubaV1.TimeSeriesPoint{Timestamp: int64(p[0]), Value: p[1]})
	}
	return pts
}

// ptsToPairs 把时间序列点拍平成 (时间戳, 值) 对，便于断言。
func ptsToPairs(pts []*ubaV1.TimeSeriesPoint) [][2]float64 {
	out := make([][2]float64, 0, len(pts))
	for _, p := range pts {
		out = append(out, [2]float64{float64(p.GetTimestamp()), p.GetValue()})
	}
	return out
}

func TestDerefStr(t *testing.T) {
	// nil 指针安全返回空串（各请求字段都是可选指针，这是最常见路径）。
	if got := derefStr(nil); got != "" {
		t.Fatalf("derefStr(nil) = %q, want empty", got)
	}
	// 正常解引用。
	s := "ios"
	if got := derefStr(&s); got != "ios" {
		t.Fatalf("derefStr(&s) = %q, want %q", got, s)
	}
	// 指向空串的指针也应返回空串。
	if got := derefStr(ptrString("")); got != "" {
		t.Fatalf("derefStr(empty ptr) = %q, want empty", got)
	}
}

func TestPtrString(t *testing.T) {
	p := ptrString("abc")
	if p == nil || *p != "abc" {
		t.Fatalf("ptrString(%q) 解引用不等于原值", "abc")
	}
	// 两次调用必须返回独立指针，不能共享底层数组。
	p1, p2 := ptrString("x"), ptrString("x")
	if p1 == p2 {
		t.Fatalf("ptrString 两次调用不应返回同一指针")
	}
}

func TestParseBucketMs(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int64
	}{
		// 空串直接短路返回 0。
		{"empty", "", 0},
		// 日粒度桶：DATE_FORMAT %Y-%m-%d 的产物（time.Parse 按 UTC 解析）。
		{"date only", "2024-01-15", time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC).UnixMilli()},
		// 小时粒度桶：%Y-%m-%d %H:00:00 的产物。
		{"datetime", "2024-01-15 10:30:45", time.Date(2024, 1, 15, 10, 30, 45, 0, time.UTC).UnixMilli()},
		// 乱码解析失败返回 0，不应 panic。
		{"garbage", "not-a-date", 0},
		// 非法月份，Go time.Parse 会拒绝。
		{"invalid month", "2024-13-01", 0},
		// ISO 8601 带 T 的格式不在支持列表内。
		{"iso with T", "2024-01-15T10:00:00", 0},
		// 记录现状（疑似缺陷，见报告）：注释声称支持周格式 %x-W%v，
		// 但两个 layout 都无法解析 "2024-W03"，周粒度趋势的所有桶时间戳会退化为 0。
		{"week format unsupported", "2024-W03", 0},
	}
	for _, c := range cases {
		if got := parseBucketMs(c.in); got != c.want {
			t.Fatalf("[%s] parseBucketMs(%q) = %d, want %d", c.name, c.in, got, c.want)
		}
	}
}

func TestNormTimeRange(t *testing.T) {
	// nil TimeRange：proto getter 对 nil 返回 0，应等价于零值输入，不 panic。
	before := time.Now().UnixMilli()
	s, e := normTimeRange(nil)
	after := time.Now().UnixMilli()
	if e < before || e > after {
		t.Fatalf("normTimeRange(nil) end=%d 应落在 [%d,%d]", e, before, after)
	}
	if s != e-sevenDayMs {
		t.Fatalf("normTimeRange(nil) start=%d, want end-7d=%d", s, e-sevenDayMs)
	}

	cases := []struct {
		name      string
		tr        *ubaV1.TimeRange
		wantStart int64
		wantEnd   int64
		endIsNow  bool // end 由 time.Now() 兜底，只能断言落在调用时刻的窗口内
		startKept bool // start 合法（>0 且 <= end）时应原样保留
	}{
		{
			// 合法区间原样返回，不做任何调整。
			name:      "valid range",
			tr:        &ubaV1.TimeRange{StartMs: 1000, EndMs: 2000},
			wantStart: 1000, wantEnd: 2000,
		},
		{
			// 只给 start、缺 end：end 兜底为当前时间，start 保留。
			name:     "start only, end defaults to now",
			tr:       &ubaV1.TimeRange{StartMs: before - 1000},
			endIsNow: true, startKept: true,
		},
		{
			// 只给 end、缺 start：start 兜底为 end-7d。
			name:      "end only",
			tr:        &ubaV1.TimeRange{EndMs: 5000},
			wantStart: 5000 - sevenDayMs, wantEnd: 5000,
		},
		{
			// start > end（脏数据）：end 保留，start 重置为 end-7d。
			name:      "start after end",
			tr:        &ubaV1.TimeRange{StartMs: 3000, EndMs: 2000},
			wantStart: 2000 - sevenDayMs, wantEnd: 2000,
		},
		{
			// end 为负：end 兜底为 now；start=100 因满足 start>0 且 start<=now 被"保留"，
			// 实际会形成一个横跨数十年的查询窗口（记录现状，规则一致但脏输入会产生巨窗口）。
			name:     "negative end keeps valid start",
			tr:       &ubaV1.TimeRange{StartMs: 100, EndMs: -5},
			endIsNow: true, startKept: true,
		},
	}
	for _, c := range cases {
		// 每次调用前后各取一次时间戳，把 now 的可能取值夹在窗口内。
		b := time.Now().UnixMilli()
		s, e := normTimeRange(c.tr)
		a := time.Now().UnixMilli()
		if c.endIsNow {
			if e < b || e > a {
				t.Fatalf("[%s] end=%d 应落在 [%d,%d]", c.name, e, b, a)
			}
			if c.startKept {
				if s != c.tr.GetStartMs() {
					t.Fatalf("[%s] start 应保留输入值 %d, got %d", c.name, c.tr.GetStartMs(), s)
				}
			} else if s != e-sevenDayMs {
				t.Fatalf("[%s] start=%d, want end-7d=%d", c.name, s, e-sevenDayMs)
			}
			continue
		}
		if s != c.wantStart || e != c.wantEnd {
			t.Fatalf("[%s] got (%d,%d), want (%d,%d)", c.name, s, e, c.wantStart, c.wantEnd)
		}
	}
}

func TestGranularityExpr(t *testing.T) {
	dayExpr := "DATE_FORMAT(event_time, '%Y-%m-%d')"
	cases := []struct {
		g           ubaV1.AnalyticsGranularity
		wantExpr    string
		wantIterval time.Duration
	}{
		// 四种显式粒度一一对应各自的 DATE_FORMAT 模板与桶宽。
		{ubaV1.AnalyticsGranularity_HOUR, "DATE_FORMAT(event_time, '%Y-%m-%d %H:00:00')", time.Hour},
		{ubaV1.AnalyticsGranularity_DAY, dayExpr, 24 * time.Hour},
		{ubaV1.AnalyticsGranularity_WEEK, "DATE_FORMAT(event_time, '%x-W%v')", 7 * 24 * time.Hour},
		{ubaV1.AnalyticsGranularity_MONTH, "DATE_FORMAT(event_time, '%Y-%m-01')", 30 * 24 * time.Hour},
		// 未指定：granularityExpr 恒返回天粒度（按跨度降级是 effectiveGranularity 的职责）。
		{ubaV1.AnalyticsGranularity_ANALYTICS_GRANULARITY_UNSPECIFIED, dayExpr, 24 * time.Hour},
		// 越界枚举值走 default 分支，同样返回天粒度，不允许 panic。
		{ubaV1.AnalyticsGranularity(99), dayExpr, 24 * time.Hour},
	}
	for _, c := range cases {
		expr, iv := granularityExpr(c.g)
		if expr != c.wantExpr || iv != c.wantIterval {
			t.Fatalf("granularityExpr(%v) = (%q, %v), want (%q, %v)", c.g, expr, iv, c.wantExpr, c.wantIterval)
		}
	}
}

func TestEffectiveGranularity(t *testing.T) {
	const hourMs = int64(time.Hour / time.Millisecond)
	const dayMs = int64(24 * time.Hour / time.Millisecond)
	const threeDayMs = 3 * dayMs
	cases := []struct {
		name       string
		g          ubaV1.AnalyticsGranularity
		start, end int64
		want       ubaV1.AnalyticsGranularity
	}{
		// 显式指定粒度时无条件尊重用户选择，跨度再大也不降级。
		{"explicit hour, long span", ubaV1.AnalyticsGranularity_HOUR, 0, 30 * dayMs, ubaV1.AnalyticsGranularity_HOUR},
		{"explicit day, short span", ubaV1.AnalyticsGranularity_DAY, 0, hourMs, ubaV1.AnalyticsGranularity_DAY},
		{"explicit week", ubaV1.AnalyticsGranularity_WEEK, 0, dayMs, ubaV1.AnalyticsGranularity_WEEK},
		{"explicit month", ubaV1.AnalyticsGranularity_MONTH, 0, dayMs, ubaV1.AnalyticsGranularity_MONTH},
		// 未指定 + 跨度 > 3 天 → 降级为天。
		{"unspecified, span 4d", ubaV1.AnalyticsGranularity_ANALYTICS_GRANULARITY_UNSPECIFIED, 0, 4 * dayMs, ubaV1.AnalyticsGranularity_DAY},
		// 边界：恰好 3 天不算 "> 3 天"，仍按小时。
		{"unspecified, span exactly 3d", ubaV1.AnalyticsGranularity_ANALYTICS_GRANULARITY_UNSPECIFIED, 0, threeDayMs, ubaV1.AnalyticsGranularity_HOUR},
		// 边界：3 天 + 1ms 即视为长跨度。
		{"unspecified, span 3d+1ms", ubaV1.AnalyticsGranularity_ANALYTICS_GRANULARITY_UNSPECIFIED, 0, threeDayMs + 1, ubaV1.AnalyticsGranularity_DAY},
		// 未指定 + 短跨度 → 小时。
		{"unspecified, span 1h", ubaV1.AnalyticsGranularity_ANALYTICS_GRANULARITY_UNSPECIFIED, 0, hourMs, ubaV1.AnalyticsGranularity_HOUR},
		// 起止相等（跨度 0）→ 小时。
		{"unspecified, zero span", ubaV1.AnalyticsGranularity_ANALYTICS_GRANULARITY_UNSPECIFIED, 100, 100, ubaV1.AnalyticsGranularity_HOUR},
		// 未知枚举值（非 UNSPECIFIED）原样透传，不做改写。
		{"unknown enum passthrough", ubaV1.AnalyticsGranularity(99), 0, 30 * dayMs, ubaV1.AnalyticsGranularity(99)},
	}
	for _, c := range cases {
		if got := effectiveGranularity(c.g, c.start, c.end); got != c.want {
			t.Fatalf("[%s] effectiveGranularity(%v, %d, %d) = %v, want %v", c.name, c.g, c.start, c.end, got, c.want)
		}
	}
}

func TestAlignMs(t *testing.T) {
	const hourMs = int64(time.Hour / time.Millisecond)
	cases := []struct {
		name     string
		ms       int64
		interval time.Duration
		want     int64
	}{
		// 已经对齐的时间戳原样返回。
		{"aligned", 3600000, time.Hour, 3600000},
		{"zero", 0, time.Hour, 0},
		// 不满一小时向下取整。
		{"one ms before hour", hourMs - 1, time.Hour, 0},
		{"one ms after hour", hourMs + 1, time.Hour, hourMs},
		{"one ms before 2 days", 2*86400000 - 1, 24 * time.Hour, 86400000},
		// 间隔 <= 0 时无法取模，原样返回（防御分支，避免死循环/除零）。
		{"zero interval", 123456, 0, 123456},
		{"negative interval", 123456, -time.Hour, 123456},
		{"1ms interval is identity", 123456, time.Millisecond, 123456},
		// 负时间戳：Go 整除向零截断，-1ms 对齐到 0（记录现状）。
		{"negative ts", -1, time.Hour, 0},
	}
	for _, c := range cases {
		if got := alignMs(c.ms, c.interval); got != c.want {
			t.Fatalf("[%s] alignMs(%d, %v) = %d, want %d", c.name, c.ms, c.interval, got, c.want)
		}
	}
}

func TestFillMissingBuckets(t *testing.T) {
	cases := []struct {
		name     string
		points   []*ubaV1.TimeSeriesPoint
		start    int64
		end      int64
		interval time.Duration
		want     [][2]float64
	}{
		{
			// 空输入原样返回（nil），不能 panic。
			name:   "nil points",
			points: nil,
			start:  0, end: 1000, interval: time.Second,
		},
		{
			// 间隔非法时无法推桶，原样返回输入切片。
			name:   "non-positive interval",
			points: mkPts([2]float64{0, 1}, [2]float64{1000, 2}),
			start:  0, end: 1000, interval: 0,
			want: [][2]float64{{0, 1}, {1000, 2}},
		},
		{
			// 中间缺口补零桶，保证折线连续。
			name:   "gap filled with zeros",
			points: mkPts([2]float64{0, 1}, [2]float64{2000, 2}, [2]float64{3000, 3}),
			start:  0, end: 5000, interval: time.Second,
			want: [][2]float64{{0, 1}, {1000, 0}, {2000, 2}, {3000, 3}},
		},
		{
			// 无缺口时顺序与数值都保持不变。
			name:   "no gap",
			points: mkPts([2]float64{0, 1}, [2]float64{1000, 2}),
			start:  0, end: 1000, interval: time.Second,
			want: [][2]float64{{0, 1}, {1000, 2}},
		},
		{
			// start 未对齐时游标从 alignMs(start) 开始，首点之前的桶补零。
			name:   "leading zeros with unaligned start",
			points: mkPts([2]float64{2000, 2}),
			start:  500, end: 3000, interval: time.Second,
			want: [][2]float64{{0, 0}, {1000, 0}, {2000, 2}},
		},
		{
			// 记录现状：最后一个数据点之后的尾部桶不补零（只补点与点之间的缺口）。
			name:   "trailing buckets not filled",
			points: mkPts([2]float64{0, 1}),
			start:  0, end: 3000, interval: time.Second,
			want: [][2]float64{{0, 1}},
		},
		{
			// 记录现状：时间戳超出 endMs 的点不会被丢弃，会原样追加到尾部。
			name:   "out-of-range point kept at tail",
			points: mkPts([2]float64{0, 1}, [2]float64{6000, 9}),
			start:  0, end: 5000, interval: time.Second,
			want: [][2]float64{{0, 1}, {1000, 0}, {2000, 0}, {3000, 0}, {4000, 0}, {5000, 0}, {6000, 9}},
		},
	}
	for _, c := range cases {
		got := ptsToPairs(fillMissingBuckets(c.points, c.start, c.end, c.interval))
		if c.want == nil {
			if len(got) != 0 {
				t.Fatalf("[%s] want empty, got %v", c.name, got)
			}
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Fatalf("[%s] got %v, want %v", c.name, got, c.want)
		}
	}
}

// TestFillMissingBuckets_UnalignedPointDup 记录当前实现行为（疑似缺陷，见报告）：
// 数据点时间戳未按桶对齐时（pt.Timestamp < cursor 且不等于任何 cursor），
// 循环不会推进 idx，同一个点会在后续每个桶被重复追加；
// 且主循环退出后尾部兜底循环（idx 未推进）还会把它再追加一次。
// 输入 1 个点 (ts=1500)，窗口 [0,10000]、1s 桶：期望正确行为是 3 个桶
// （0 补零、1000 补零、1500 数据点），当前实际输出 12 个点、数据点重复 10 次
// （主循环 9 次 + 尾部兜底 1 次）。
// 若修复了该缺陷，请同步把本用例改为断言正确行为。
func TestFillMissingBuckets_UnalignedPointDup(t *testing.T) {
	pts := mkPts([2]float64{1500, 5})
	got := fillMissingBuckets(pts, 0, 10000, time.Second)
	if len(got) != 12 {
		t.Fatalf("当前实现应输出 12 个点（含重复）, got %d: %v", len(got), ptsToPairs(got))
	}
	if got[0].GetTimestamp() != 0 || got[0].GetValue() != 0 {
		t.Fatalf("首桶应为 {0,0}, got %v", ptsToPairs(got[:1]))
	}
	if got[1].GetTimestamp() != 1000 || got[1].GetValue() != 0 {
		t.Fatalf("第二桶应为 {1000,0}, got %v", ptsToPairs(got[1:2]))
	}
	dups := 0
	for _, p := range got[2:] {
		if p.GetTimestamp() == 1500 && p.GetValue() == 5 {
			dups++
		}
	}
	if dups != 10 {
		t.Fatalf("未对齐数据点应被重复追加 10 次（主循环 9 次 + 尾部 1 次，记录现状）, got %d", dups)
	}
}

func TestAllowedDimension(t *testing.T) {
	// 白名单内 10 个维度全部放行，且映射后列名不变。
	for _, dim := range []string{
		"platform", "channel", "country", "app_version", "event_name",
		"event_category", "os", "network", "user_level", "vip_level",
	} {
		col, ok := allowedDimension(dim)
		if !ok || col != dim {
			t.Fatalf("allowedDimension(%q) = (%q, %v), want (%q, true)", dim, col, ok, dim)
		}
	}
	// 白名单外的输入一律拒绝：空串、未知列、大小写不匹配、注入载荷。
	for _, dim := range []string{
		"", "user_id", "PLATFORM", "Platform",
		"platform; DROP TABLE events_fact", "platform'--", "event_name || 'x'",
	} {
		if _, ok := allowedDimension(dim); ok {
			t.Fatalf("allowedDimension(%q) 应被白名单拒绝", dim)
		}
	}
}

func TestJoinUsersDim(t *testing.T) {
	// 游戏维度在 users_dim 维度表，事实表无此列，必须 JOIN。
	if !joinUsersDim("user_level") || !joinUsersDim("vip_level") {
		t.Fatalf("user_level/vip_level 应返回 true")
	}
	// 其余维度都在 events_fact 上，不需要 JOIN。
	for _, dim := range []string{"platform", "event_name", "", "user_id", "USER_LEVEL"} {
		if joinUsersDim(dim) {
			t.Fatalf("joinUsersDim(%q) 应为 false", dim)
		}
	}
}

func TestMetricExpr(t *testing.T) {
	cases := []struct {
		metric string
		want   string
	}{
		// 空串视为默认 COUNT。
		{"", "COUNT(*)"},
		{"COUNT", "COUNT(*)"},
		// 去重用户数固定作用于 user_id 列。
		{"UNIQUE_USER", "COUNT(DISTINCT user_id)"},
		// 金额类聚合强制 CAST DOUBLE + COALESCE 防 NULL。
		{"SUM_AMOUNT", "CAST(COALESCE(SUM(CAST(amount AS DOUBLE)), 0) AS DOUBLE)"},
		{"AVG_AMOUNT", "AVG(CAST(amount AS DOUBLE))"},
		// 人均次数：NULLIF 防除零。
		{"PER_USER", "CAST(COUNT(*) AS DOUBLE) / NULLIF(COUNT(DISTINCT user_id), 0)"},
	}
	for _, c := range cases {
		got, err := metricExpr(c.metric, "")
		if err != nil {
			t.Fatalf("metricExpr(%q) 不应报错: %v", c.metric, err)
		}
		if got != c.want {
			t.Fatalf("metricExpr(%q) = %q, want %q", c.metric, got, c.want)
		}
	}
	// 非法指标一律拒绝：未知名、小写变体、注入载荷、带参调用。
	for _, metric := range []string{
		"SUM(amount)", "count", "Count", "COUNT(1); DROP TABLE events_fact",
		"UNIQUE_USER--", " ",
	} {
		if _, err := metricExpr(metric, "amount"); err == nil {
			t.Fatalf("metricExpr(%q) 应返回错误", metric)
		}
	}
}

func TestOperatorSQL(t *testing.T) {
	cases := []struct {
		op      ubaV1.PropertyFilter_Operator
		wantSQL string
		wantIn  bool
	}{
		// UNSPECIFIED 兼容旧客户端，等价于 EQ。
		{ubaV1.PropertyFilter_OPERATOR_UNSPECIFIED, "=", false},
		{ubaV1.PropertyFilter_EQ, "=", false},
		{ubaV1.PropertyFilter_NEQ, "<>", false},
		// CONTAINS 返回花括号模板（由 buildFilterWhere 填充，见对应测试）。
		{ubaV1.PropertyFilter_CONTAINS, "position({} IN {}) > 0", false},
		// IN 是多值算子，buildFilterWhere 需按 isIn 走占位符列表分支。
		{ubaV1.PropertyFilter_IN, "IN", true},
		{ubaV1.PropertyFilter_GT, ">", false},
		{ubaV1.PropertyFilter_GTE, ">=", false},
		{ubaV1.PropertyFilter_LT, "<", false},
		{ubaV1.PropertyFilter_LTE, "<=", false},
	}
	for _, c := range cases {
		sql, isIn, err := operatorSQL(c.op)
		if err != nil {
			t.Fatalf("operatorSQL(%v) 不应报错: %v", c.op, err)
		}
		if sql != c.wantSQL || isIn != c.wantIn {
			t.Fatalf("operatorSQL(%v) = (%q, %v), want (%q, %v)", c.op, sql, isIn, c.wantSQL, c.wantIn)
		}
	}
	// 越界枚举值必须报错，不允许拼进 SQL。
	for _, op := range []ubaV1.PropertyFilter_Operator{99, -1} {
		if _, _, err := operatorSQL(op); err == nil {
			t.Fatalf("operatorSQL(%d) 应返回错误", op)
		}
	}
}

func TestResolveFilterColumn(t *testing.T) {
	cases := []struct {
		name     string
		scope    ubaV1.PropertyFilter_FieldScope
		field    string
		wantCol  string
		wantJoin bool
		wantErr  bool
	}{
		// UNSPECIFIED 按 DIMENSION 处理（向后兼容旧客户端）。
		{"unspecified scope whitelist dim", ubaV1.PropertyFilter_FIELD_SCOPE_UNSPECIFIED, "platform", "platform", false, false},
		// 标准维度列直接返回列名。
		{"dimension whitelist dim", ubaV1.PropertyFilter_DIMENSION, "platform", "platform", false, false},
		// 游戏维度在 users_dim，返回带 u. 前缀并要求 JOIN。
		{"dimension user_level needs join", ubaV1.PropertyFilter_DIMENSION, "user_level", "u.user_level", true, false},
		{"dimension vip_level needs join", ubaV1.PropertyFilter_DIMENSION, "vip_level", "u.vip_level", true, false},
		// Map 键走 context['key'] / properties['key'] / metrics['key']。
		{"context key", ubaV1.PropertyFilter_EVENT_CONTEXT, "variant", "context['variant']", false, false},
		{"property key with digits/underscore", ubaV1.PropertyFilter_EVENT_PROPERTY, "item_id_2", "properties['item_id_2']", false, false},
		{"metric key", ubaV1.PropertyFilter_EVENT_METRIC, "coin", "metrics['coin']", false, false},

		// ---- 拒绝类：白名单外维度、非法 Map 键（含注入载荷）----
		{"dimension not whitelisted", ubaV1.PropertyFilter_DIMENSION, "user_id", "", false, true},
		{"dimension case sensitive", ubaV1.PropertyFilter_DIMENSION, "PLATFORM", "", false, true},
		{"dimension injection", ubaV1.PropertyFilter_DIMENSION, "1; DROP TABLE events_fact", "", false, true},
		{"dimension empty", ubaV1.PropertyFilter_DIMENSION, "", "", false, true},
		// 注入必测：属性键里的引号/分号/注释都会被 mapKeyRe 拦截。
		{"property injection", ubaV1.PropertyFilter_EVENT_PROPERTY, "x'); DROP TABLE--", "", false, true},
		{"property space", ubaV1.PropertyFilter_EVENT_PROPERTY, "a b", "", false, true},
		{"property empty", ubaV1.PropertyFilter_EVENT_PROPERTY, "", "", false, true},
		{"property non-ascii", ubaV1.PropertyFilter_EVENT_PROPERTY, "键名", "", false, true},
		{"context injection", ubaV1.PropertyFilter_EVENT_CONTEXT, "x'--", "", false, true},
		{"context empty", ubaV1.PropertyFilter_EVENT_CONTEXT, "", "", false, true},
		{"metric injection", ubaV1.PropertyFilter_EVENT_METRIC, "coin'--", "", false, true},
		// 未知 scope 报错，不允许落到任何列。
		{"unknown scope", ubaV1.PropertyFilter_FieldScope(99), "platform", "", false, true},
	}
	for _, c := range cases {
		col, join, err := resolveFilterColumn(c.scope, c.field)
		if c.wantErr {
			if err == nil {
				t.Fatalf("[%s] resolveFilterColumn(%v, %q) 应返回错误", c.name, c.scope, c.field)
			}
			continue
		}
		if err != nil {
			t.Fatalf("[%s] resolveFilterColumn(%v, %q) 不应报错: %v", c.name, c.scope, c.field, err)
		}
		if col != c.wantCol || join != c.wantJoin {
			t.Fatalf("[%s] resolveFilterColumn(%v, %q) = (%q, %v), want (%q, %v)", c.name, c.scope, c.field, col, join, c.wantCol, c.wantJoin)
		}
	}
}

func TestBuildFilterWhere(t *testing.T) {
	dim := func(field string, op ubaV1.PropertyFilter_Operator, values ...string) *ubaV1.PropertyFilter {
		return &ubaV1.PropertyFilter{Scope: ubaV1.PropertyFilter_DIMENSION, Field: field, Op: op, Values: values}
	}
	cases := []struct {
		name        string
		group       *ubaV1.FilterGroup
		wantClauses []string
		wantArgs    []any
		wantJoin    bool
		wantErr     bool
	}{
		{
			// nil / 空 group：无过滤条件，无需 JOIN，不报错。
			name:  "nil group",
			group: nil,
		},
		{
			name:  "empty group",
			group: &ubaV1.FilterGroup{},
		},
		{
			// 组内的 nil 过滤项应被跳过而不是 panic。
			name:  "nil filter entry skipped",
			group: &ubaV1.FilterGroup{Filters: []*ubaV1.PropertyFilter{nil}},
		},
		{
			// 单条等值过滤：列名 + 占位符，参数顺序与 clause 一一对应。
			name:        "single EQ",
			group:       &ubaV1.FilterGroup{Filters: []*ubaV1.PropertyFilter{dim("platform", ubaV1.PropertyFilter_EQ, "ios")}},
			wantClauses: []string{"platform = ?"},
			wantArgs:    []any{"ios"},
		},
		{
			// UNSPECIFIED 算子按 EQ 处理（旧客户端兼容）。
			name:        "unspecified op equals EQ",
			group:       &ubaV1.FilterGroup{Filters: []*ubaV1.PropertyFilter{dim("platform", ubaV1.PropertyFilter_OPERATOR_UNSPECIFIED, "ios")}},
			wantClauses: []string{"platform = ?"},
			wantArgs:    []any{"ios"},
		},
		{
			// NEQ。
			name:        "NEQ",
			group:       &ubaV1.FilterGroup{Filters: []*ubaV1.PropertyFilter{dim("country", ubaV1.PropertyFilter_NEQ, "CN")}},
			wantClauses: []string{"country <> ?"},
			wantArgs:    []any{"CN"},
		},
		{
			// 游戏维度触发 users_dim JOIN 标记。
			name:        "join dimension",
			group:       &ubaV1.FilterGroup{Filters: []*ubaV1.PropertyFilter{dim("user_level", ubaV1.PropertyFilter_GTE, "10")}},
			wantClauses: []string{"u.user_level >= ?"},
			wantArgs:    []any{"10"},
			wantJoin:    true,
		},
		{
			// IN 多值：占位符数量与值数量一致，顺序保持。
			name:        "IN multi values",
			group:       &ubaV1.FilterGroup{Filters: []*ubaV1.PropertyFilter{dim("platform", ubaV1.PropertyFilter_IN, "ios", "android")}},
			wantClauses: []string{"platform IN (?, ?)"},
			wantArgs:    []any{"ios", "android"},
		},
		{
			// IN 但值为空：跳过该条件（等于不过滤），不产生空 IN ()。
			name:  "IN with empty values skipped",
			group: &ubaV1.FilterGroup{Filters: []*ubaV1.PropertyFilter{dim("platform", ubaV1.PropertyFilter_IN)}},
		},
		{
			// 非 IN 算子值为空同样跳过。
			name:  "EQ with empty values skipped",
			group: &ubaV1.FilterGroup{Filters: []*ubaV1.PropertyFilter{dim("platform", ubaV1.PropertyFilter_EQ)}},
		},
		{
			// 多条过滤各自成 clause，参数按条件顺序拼接。
			name: "multiple filters AND",
			group: &ubaV1.FilterGroup{Filters: []*ubaV1.PropertyFilter{
				dim("platform", ubaV1.PropertyFilter_EQ, "ios"),
				dim("country", ubaV1.PropertyFilter_EQ, "CN"),
			}},
			wantClauses: []string{"platform = ?", "country = ?"},
			wantArgs:    []any{"ios", "CN"},
		},
		{
			// 空值条件被跳过后，其余条件照常输出。
			name: "skipped filter mixed with valid",
			group: &ubaV1.FilterGroup{Filters: []*ubaV1.PropertyFilter{
				dim("platform", ubaV1.PropertyFilter_IN),
				dim("channel", ubaV1.PropertyFilter_EQ, "google"),
			}},
			wantClauses: []string{"channel = ?"},
			wantArgs:    []any{"google"},
		},
		{
			// 事件属性 Map 键过滤。
			name: "event property filter",
			group: &ubaV1.FilterGroup{Filters: []*ubaV1.PropertyFilter{{
				Scope:  ubaV1.PropertyFilter_EVENT_PROPERTY,
				Field:  "amount",
				Op:     ubaV1.PropertyFilter_GT,
				Values: []string{"100"},
			}}},
			wantClauses: []string{"properties['amount'] > ?"},
			wantArgs:    []any{"100"},
		},
		// ---- 错误类：非法维度 / 非法算子 / 注入键名 ----
		{
			name:    "invalid dimension",
			group:   &ubaV1.FilterGroup{Filters: []*ubaV1.PropertyFilter{dim("evil;--", ubaV1.PropertyFilter_EQ, "x")}},
			wantErr: true,
		},
		{
			name:    "invalid operator",
			group:   &ubaV1.FilterGroup{Filters: []*ubaV1.PropertyFilter{dim("platform", ubaV1.PropertyFilter_Operator(99), "x")}},
			wantErr: true,
		},
		{
			name: "property key injection rejected",
			group: &ubaV1.FilterGroup{Filters: []*ubaV1.PropertyFilter{{
				Scope:  ubaV1.PropertyFilter_EVENT_PROPERTY,
				Field:  "x'); DROP TABLE events_fact--",
				Op:     ubaV1.PropertyFilter_EQ,
				Values: []string{"1"},
			}}},
			wantErr: true,
		},
	}
	for _, c := range cases {
		clauses, args, needJoin, err := buildFilterWhere(c.group)
		if c.wantErr {
			if err == nil {
				t.Fatalf("[%s] buildFilterWhere 应返回错误", c.name)
			}
			continue
		}
		if err != nil {
			t.Fatalf("[%s] buildFilterWhere 不应报错: %v", c.name, err)
		}
		if len(c.wantClauses) == 0 {
			if len(clauses) != 0 || len(args) != 0 {
				t.Fatalf("[%s] want empty clauses/args, got %v / %v", c.name, clauses, args)
			}
		} else {
			if !reflect.DeepEqual(clauses, c.wantClauses) {
				t.Fatalf("[%s] clauses = %v, want %v", c.name, clauses, c.wantClauses)
			}
			if !reflect.DeepEqual(args, c.wantArgs) {
				t.Fatalf("[%s] args = %v, want %v", c.name, args, c.wantArgs)
			}
		}
		if needJoin != c.wantJoin {
			t.Fatalf("[%s] needJoin = %v, want %v", c.name, needJoin, c.wantJoin)
		}
	}
}

// TestBuildFilterWhere_ContainsTemplate 记录当前实现行为（疑似缺陷，见报告）：
// operatorSQL(CONTAINS) 返回的花括号模板 "position({} IN {}) > 0" 没有 fmt 的 % 动词，
// fmt.Sprintf(opSQL, "?", col) 不会做任何替换，还会追加 "%!(EXTRA ...)" 尾巴，
// 导致 clause 中既没有 ? 占位符、又混入 fmt 调试文本，执行时必然 SQL 报错。
// 本用例锁定该现状；若修复为 %s 模板，请同步更新断言为 "position(? IN event_name) > 0"。
func TestBuildFilterWhere_ContainsTemplate(t *testing.T) {
	group := &ubaV1.FilterGroup{Filters: []*ubaV1.PropertyFilter{{
		Scope:  ubaV1.PropertyFilter_DIMENSION,
		Field:  "event_name",
		Op:     ubaV1.PropertyFilter_CONTAINS,
		Values: []string{"pay"},
	}}}
	clauses, args, needJoin, err := buildFilterWhere(group)
	if err != nil {
		t.Fatalf("buildFilterWhere 不应报错: %v", err)
	}
	want := "position({} IN {}) > 0%!(EXTRA string=?, string=event_name)"
	if len(clauses) != 1 || clauses[0] != want {
		t.Fatalf("CONTAINS clause = %q, want 记录现状 %q", clauses, want)
	}
	if !reflect.DeepEqual(args, []any{"pay"}) {
		t.Fatalf("CONTAINS args = %v, want [pay]", args)
	}
	if needJoin {
		t.Fatalf("CONTAINS 普通维度不应触发 JOIN")
	}
}

func TestDimensionColumnExpr(t *testing.T) {
	// 普通维度：直接返回事实表列名，无需 JOIN。
	col, join, err := dimensionColumnExpr("platform")
	if err != nil || col != "platform" || join {
		t.Fatalf("dimensionColumnExpr(platform) = (%q, %v, %v), want (platform, false, nil)", col, join, err)
	}
	// 游戏维度：带 u. 前缀并要求 JOIN users_dim。
	col, join, err = dimensionColumnExpr("vip_level")
	if err != nil || col != "u.vip_level" || !join {
		t.Fatalf("dimensionColumnExpr(vip_level) = (%q, %v, %v), want (u.vip_level, true, nil)", col, join, err)
	}
	// 白名单外的维度（含注入载荷）必须报错。
	for _, dim := range []string{"user_id", "DROP", "platform;--", ""} {
		if _, _, err := dimensionColumnExpr(dim); err == nil {
			t.Fatalf("dimensionColumnExpr(%q) 应返回错误", dim)
		}
	}
}

func TestSeriesNameOf(t *testing.T) {
	// nil 指针安全（proto getter 兜底），回退到默认名。
	if got := seriesNameOf(nil); got != "全部事件" {
		t.Fatalf("seriesNameOf(nil) = %q, want 全部事件", got)
	}
	// 无任何命名信息：默认序列名。
	if got := seriesNameOf(&ubaV1.EventQuery{}); got != "全部事件" {
		t.Fatalf("seriesNameOf(empty) = %q, want 全部事件", got)
	}
	// 只有事件名：以事件名作为序列名。
	if got := seriesNameOf(&ubaV1.EventQuery{EventName: ptrString("pay_click")}); got != "pay_click" {
		t.Fatalf("seriesNameOf(event only) = %q, want pay_click", got)
	}
	// 显示名优先级高于事件名。
	q := &ubaV1.EventQuery{EventName: ptrString("pay_click"), DisplayName: ptrString("支付点击")}
	if got := seriesNameOf(q); got != "支付点击" {
		t.Fatalf("seriesNameOf(display wins) = %q, want 支付点击", got)
	}
}

func TestTopKeysByValue(t *testing.T) {
	// nil / 空 map 返回空切片。
	if got := topKeysByValue(nil, 3); len(got) != 0 {
		t.Fatalf("topKeysByValue(nil) = %v, want empty", got)
	}
	if got := topKeysByValue(map[string]float64{}, 3); len(got) != 0 {
		t.Fatalf("topKeysByValue(empty) = %v, want empty", got)
	}
	// n <= 0 返回空切片，不 panic。
	m := map[string]float64{"a": 1}
	if got := topKeysByValue(m, 0); len(got) != 0 {
		t.Fatalf("topKeysByValue(m, 0) = %v, want empty", got)
	}
	// n 超过元素个数时返回全部，并按值降序。
	all := map[string]float64{"a": 1, "b": 5, "c": 3, "d": 2, "e": 4}
	wantAll := []string{"b", "e", "c", "d", "a"}
	if got := topKeysByValue(all, 10); !reflect.DeepEqual(got, wantAll) {
		t.Fatalf("topKeysByValue(all, 10) = %v, want %v", got, wantAll)
	}
	// n 小于元素个数时取降序前 n 个。
	wantTop3 := []string{"b", "e", "c"}
	if got := topKeysByValue(all, 3); !reflect.DeepEqual(got, wantTop3) {
		t.Fatalf("topKeysByValue(all, 3) = %v, want %v", got, wantTop3)
	}
	// 负值也参与排序（降序）。
	neg := map[string]float64{"x": -1, "y": -5, "z": -3}
	if got := topKeysByValue(neg, 3); !reflect.DeepEqual(got, []string{"x", "z", "y"}) {
		t.Fatalf("topKeysByValue(neg) = %v, want [x z y]", got)
	}
}

// TestTopKeysByValue_NegativeN_Panic 记录当前实现行为（疑似缺陷，见报告）：
// topKeysByValue 直接把 n 用作 make 的 cap（analytics_repo.go: out := make([]string, 0, n)），
// n 为负数时触发 "makeslice: cap out of range" panic。
// 若调用方把请求里的 topN 原样传入（如 Distribution 的 topN），负值会导致整个请求 500。
// 若实现已修复（负数返回空切片或按 0 处理），请把本用例改为断言正常返回空切片。
func TestTopKeysByValue_NegativeN_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("topKeysByValue(m, -1) 当前应触发 makeslice panic；若已修复，请同步更新本用例")
		}
	}()
	_ = topKeysByValue(map[string]float64{"a": 1}, -1)
}

func TestToU32s(t *testing.T) {
	// nil / 空 → 空切片。
	if got := toU32s(nil); len(got) != 0 {
		t.Fatalf("toU32s(nil) = %v, want empty", got)
	}
	// 正常转换保序。
	if got := toU32s([]int64{1, 2, 3}); !reflect.DeepEqual(got, []uint32{1, 2, 3}) {
		t.Fatalf("toU32s([1,2,3]) = %v", got)
	}
	// 记录现状：负数与超界值按 Go 截断语义回绕（调用方只传合法 ID，防御性记录）。
	if got := toU32s([]int64{-1}); !reflect.DeepEqual(got, []uint32{4294967295}) {
		t.Fatalf("toU32s([-1]) = %v, want [4294967295]", got)
	}
	if got := toU32s([]int64{1 << 32}); !reflect.DeepEqual(got, []uint32{0}) {
		t.Fatalf("toU32s([2^32]) = %v, want [0]", got)
	}
}

func TestU32p(t *testing.T) {
	// nil 指针透传 nil。
	if got := u32p(nil); got != nil {
		t.Fatalf("u32p(nil) = %v, want nil", got)
	}
	// 正常转换。
	v := int64(7)
	if got := u32p(&v); got == nil || *got != 7 {
		t.Fatalf("u32p(&7) = %v, want 7", got)
	}
	// 记录现状：负值回绕。
	neg := int64(-1)
	if got := u32p(&neg); got == nil || *got != 4294967295 {
		t.Fatalf("u32p(&-1) = %v, want 4294967295", got)
	}
}
