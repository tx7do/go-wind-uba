package olapcfg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalize(t *testing.T) {
	Set(Doris)
	t.Cleanup(func() { Set(Doris) })

	for in, want := range map[string]Engine{
		"doris":       Doris,
		"ClickHouse":  ClickHouse,
		" clickhouse": ClickHouse,
		"CH":          Doris, // 未知值 fail-safe
		"":            Doris, // 空值 fail-safe
		"sqlite":      Doris, // 未知引擎回退
	} {
		Set(Engine(in))
		if got := Current(); got != want {
			t.Errorf("normalize(%q) = %v, want %v", in, got, want)
		}
		if UseClickHouse() != (want == ClickHouse) {
			t.Errorf("UseClickHouse() disagrees with %v", want)
		}
	}
}

func TestLoadFromDirs(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ConfigFile),
		[]byte("olap:\n  engine: clickhouse\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// 配置文件生效
	if got := loadWith("", []string{dir}); got != ClickHouse {
		t.Fatalf("yaml config = %v, want clickhouse", got)
	}
	// 环境变量优先于文件
	if got := loadWith("doris", []string{dir}); got != Doris {
		t.Fatalf("env override = %v, want doris", got)
	}

	// 空目录回退默认
	if got := loadWith("", []string{t.TempDir()}); got != Doris {
		t.Fatalf("empty dirs = %v, want default doris", got)
	}

	// 多目录:命中即用,不向后覆盖
	dir2 := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir2, ConfigFile), []byte("olap:\n  engine: clickhouse\n"), 0o644)
	if got := loadWith("", []string{dir, dir2}); got != ClickHouse {
		t.Fatalf("multi dirs = %v, want clickhouse", got)
	}
}
