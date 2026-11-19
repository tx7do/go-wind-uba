package doris

import (
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/structpb"
)

// TestValidateAdhocQuery_Boundary 补充自助查询静态校验的边界用例。
// 先读 sql_query.go 确认实现语义：只做 ①非空 ②允许多余一个尾分号 ③禁注释标记（; -- /* */ #）
// ④首关键字（按空白切分后第一个词）大小写归一后在只读白名单内——不做更深的语法解析。
func TestValidateAdhocQuery_Boundary(t *testing.T) {
	// 按当前实现语义应放行的输入。
	allowed := []struct {
		q   string
		why string
	}{
		// 反引号标识符不是禁止标记，放行（记录现状：反引号内容不做校验）。
		{"SELECT * FROM `events_fact`", "backtick identifier"},
		// strings.Fields 按所有空白（含换行）切分，换行不影响首关键字判定。
		{"select 1\nfrom events_fact", "newline inside"},
		// 首关键字大小写归一（ToUpper）后比较，大小写混合合法。
		{"SeLeCt 1", "mixed case keyword"},
		// UNION 不是禁止标记，只校验首关键字（记录现状：UNION 子查询放行）。
		{"select 1 union select 2", "union allowed"},
		// 只有首个 token 参与白名单比较，后续 token 是啥不关心。
		{"EXPLAIN FORMAT TREE SELECT 1", "explain with format"},
		// 单个关键字也算合法语句。
		{"select", "bare keyword"},
		// 尾分号 + 尾随空白：先 TrimSpace 再 TrimSuffix，仍放行。
		{"SHOW TABLES ; ", "trailing semicolon with spaces"},
	}
	for _, c := range allowed {
		if err := ValidateAdhocQuery(c.q); err != nil {
			t.Fatalf("query %q (%s) should be allowed, got %v", c.q, c.why, err)
		}
	}

	// 按当前实现语义应拒绝的输入。
	rejected := []struct {
		q   string
		why string
	}{
		// 前缀注入：SELECTX != SELECT，不在白名单。
		{"SELECTX 1", "prefix injection"},
		{"SeLeCtX 1", "mixed case prefix injection"},
		{"SELECTING x", "prefix injection 2"},
		// 字符串字面量里的分号同样被拒——过严但安全（防多语句优先）。
		{"select 'a;b'", "semicolon inside string literal"},
		// 组合注释标记。
		{"select 1 /*x*/ --y", "combined comments"},
		// 嵌套块注释只查 "/*" 与 "*/" 标记，命中即拒。
		{"select 1 /*/ ", "block comment marker"},
		// 尾部双分号：TrimSuffix 只去掉一个，剩余 ";" 命中多语句。
		{"SELECT 1;;", "double trailing semicolon"},
		// 分号在中间：多语句。
		{"select 1 ;\nselect 2", "mid semicolon"},
	}
	for _, c := range rejected {
		if err := ValidateAdhocQuery(c.q); err == nil {
			t.Fatalf("query %q (%s) should be rejected", c.q, c.why)
		}
	}
}

// TestValidateAdhocQuery_SemicolonOnly_Panic 记录当前实现行为（疑似缺陷，见报告）：
// 输入仅含 ";" 时：非空检查通过 → TrimSuffix 后变成空串 → 多语句检查通过 →
// strings.Fields("")[0] 触发切片越界 panic。
// 若实现已修复为返回 BadRequest 错误，请把本用例改为断言 err != nil 且 recover 为 nil。
func TestValidateAdhocQuery_SemicolonOnly_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("输入 %q 当前应触发 panic(strings.Fields 空切片越界)；若已修复为返回错误，请同步更新本用例", ";")
		}
	}()
	_ = ValidateAdhocQuery(";")
}

func TestToStructValue(t *testing.T) {
	cases := []struct {
		name  string
		in    any
		check func(*structpb.Value) bool
	}{
		// SQL NULL → protobuf NullValue。
		{"nil", nil, func(v *structpb.Value) bool { return v.GetNullValue() == structpb.NullValue_NULL_VALUE }},
		// 字符串原样。
		{"string", "ios", func(v *structpb.Value) bool { return v.GetStringValue() == "ios" }},
		// 数值类走 NumberValue。
		{"int64", int64(42), func(v *structpb.Value) bool { return v.GetNumberValue() == 42 }},
		{"uint32", uint32(7), func(v *structpb.Value) bool { return v.GetNumberValue() == 7 }},
		{"float64", 1.5, func(v *structpb.Value) bool { return v.GetNumberValue() == 1.5 }},
		{"bool", true, func(v *structpb.Value) bool { return v.GetBoolValue() }},
		// []byte 由 toStructValue 自行命中分支并 string() 原样转换
		// （注意：与 structpb.NewValue 直接处理 []byte 的 base64 编码语义不同）。
		{"bytes raw string", []byte("abc"), func(v *structpb.Value) bool {
			return v.GetStringValue() == "abc"
		}},
		// time.Time 在 toStructValue 里自行格式化为 RFC3339Nano 字符串。
		{"time", time.Date(2024, 1, 15, 10, 30, 0, 123456789, time.UTC), func(v *structpb.Value) bool {
			return v.GetStringValue() == "2024-01-15T10:30:00.123456789Z"
		}},
		// map/list 递归转 Struct/List。
		{"map", map[string]any{"a": int64(1)}, func(v *structpb.Value) bool {
			return v.GetStructValue().GetFields()["a"].GetNumberValue() == 1
		}},
		{"list", []any{"x", int64(2)}, func(v *structpb.Value) bool {
			return v.GetListValue().GetValues()[0].GetStringValue() == "x" &&
				v.GetListValue().GetValues()[1].GetNumberValue() == 2
		}},
	}
	for _, c := range cases {
		v, err := toStructValue(c.in)
		if err != nil {
			t.Fatalf("[%s] toStructValue(%v) 不应报错: %v", c.name, c.in, err)
		}
		if !c.check(v) {
			t.Fatalf("[%s] toStructValue(%v) 转换结果不符合预期: %v", c.name, c.in, v)
		}
	}
	// 驱动返回的不支持类型（如 chan）必须报错而不是 panic。
	if _, err := toStructValue(make(chan int)); err == nil {
		t.Fatalf("toStructValue(chan) 应返回错误")
	}
	// 记录现状：含非法 UTF-8 的 string 会被 structpb 拒绝。
	if _, err := toStructValue(string([]byte{0xff, 0xfe})); err == nil {
		t.Fatalf("toStructValue(invalid utf8 string) 应返回错误")
	}
}

// 与 ValidateAdhocQuery 相关的哨兵：确认错误信息里带白名单关键字，便于前端提示。
func TestValidateAdhocQuery_ErrorMentionsWhitelist(t *testing.T) {
	err := ValidateAdhocQuery("INSERT INTO t VALUES(1)")
	if err == nil {
		t.Fatalf("INSERT 应被拒绝")
	}
	// 错误文案应包含被拒首关键字，便于定位。
	if !strings.Contains(err.Error(), "INSERT") {
		t.Fatalf("错误信息应包含首关键字 INSERT, got %v", err)
	}
}
