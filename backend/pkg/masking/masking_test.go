package masking

import (
	"context"
	"testing"
)

type stubStrategy struct {
	masks map[string]Kind // "resource.field" → kind
	// seenTargets 记录每次判定收到的目标归属者(验证接缝传递)。
	seenTargets []int64
}

func (s *stubStrategy) FieldMask(_ context.Context, resource, field string, targetOwnerID int64) Kind {
	s.seenTargets = append(s.seenTargets, targetOwnerID)
	return s.masks[resource+"."+field]
}

func TestNoStrategyAlwaysPassthrough(t *testing.T) {
	SetStrategy(nil)
	t.Cleanup(func() { SetStrategy(nil) })

	v := "alice@uba.dev"
	if got := Maybe(context.Background(), ResourceUser, "email", &v); got != &v || *got != v {
		t.Fatalf("Maybe() without strategy changed value: %q", *got)
	}
	if Available() {
		t.Fatal("Available() = true without strategy")
	}
}

func TestApplyKinds(t *testing.T) {
	cases := []struct {
		kind Kind
		in   string
		want string
	}{
		{KindNone, "alice@uba.dev", "alice@uba.dev"},
		{KindEmail, "alice@uba.dev", "a***@uba.dev"},
		{KindEmail, "a@uba.dev", "a***@uba.dev"},
		{KindEmail, "not-an-email", "***"}, // 无 @ 全遮蔽
		{KindPhone, "13812345678", "138****5678"},
		{KindPhone, "12345", "***"}, // 过短全遮蔽
		{KindFull, "anything", "***"},
		{Kind("unknown"), "anything", "***"}, // 未知形态 fail-closed
	}
	for _, c := range cases {
		if got := Apply(c.kind, c.in); got != c.want {
			t.Errorf("Apply(%q, %q) = %q, want %q", c.kind, c.in, got, c.want)
		}
	}
}

func TestMaybeWithStrategy(t *testing.T) {
	stub := &stubStrategy{masks: map[string]Kind{
		"user.email": KindEmail,
		"user.phone": KindPhone,
	}}
	SetStrategy(stub)
	t.Cleanup(func() { SetStrategy(nil) })

	ctx := context.Background()
	email := "alice@uba.dev"
	mobile := "13812345678"
	empty := ""
	untouched := "plain-value"

	if got := Maybe(ctx, ResourceUser, "email", &email); *got != "a***@uba.dev" {
		t.Fatalf("masked email = %q", *got)
	}
	if got := Maybe(ctx, ResourceUser, "phone", &mobile); *got != "138****5678" {
		t.Fatalf("masked mobile = %q", *got)
	}
	if got := Maybe(ctx, ResourceUser, "mobile", &mobile); *got != mobile { // 未配置的字段
		t.Fatalf("unconfigured field changed: %q", *got)
	}
	if got := Maybe(ctx, ResourceUser, "email", &empty); got != &empty || *got != "" { // 空值不动
		t.Fatalf("empty value changed: %q", *got)
	}
	if got := Maybe(ctx, ResourceUser, "email", nil); got != nil { // nil 安全
		t.Fatal("nil value changed")
	}
	if got := Maybe(ctx, ResourceUser, "unrelated", &untouched); *got != untouched {
		t.Fatalf("strategy-free field changed: %q", *got)
	}

	// Maybe 不带目标归属:判定收到 0
	if stub.seenTargets[len(stub.seenTargets)-1] != 0 {
		t.Fatalf("Maybe target = %d, want 0", stub.seenTargets[len(stub.seenTargets)-1])
	}

	// MaybeFor 透传目标归属(供策略做"看自己不脱敏")
	if got := MaybeFor(ctx, ResourceUser, "email", &email, 42); *got != "a***@uba.dev" {
		t.Fatalf("masked email with target = %q", *got)
	}
	if stub.seenTargets[len(stub.seenTargets)-1] != 42 {
		t.Fatalf("MaybeFor target = %d, want 42", stub.seenTargets[len(stub.seenTargets)-1])
	}
}
