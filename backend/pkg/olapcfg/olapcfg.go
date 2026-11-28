// Package olapcfg 解析 OLAP 引擎的部署配置:Doris / ClickHouse 二选一。
//
// 决定顺序:环境变量 UBA_OLAP_ENGINE > 配置目录下 olap.yaml 的 olap.engine
// > 默认 Doris(与历史编译期默认一致,fail-safe)。
//
// 进程启动时一次性装载(data 包初始化时取用);配置非法时回退 Doris
// 并向 stderr 输出告警。本包刻意只依赖标准库与 yaml,避免引入环。
package olapcfg

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Engine OLAP 引擎。
type Engine string

const (
	// Doris Apache Doris(默认)。
	Doris Engine = "doris"
	// ClickHouse ClickHouse。
	ClickHouse Engine = "clickhouse"
)

// EnvEngine 引擎选择的环境变量,优先于配置文件。
const EnvEngine = "UBA_OLAP_ENGINE"

// ConfigFile 引擎配置文件名,位于服务配置目录下(与 -c / UBA_CONFIGS 同目录集)。
const ConfigFile = "olap.yaml"

var current = load()

// Current 当前生效的 OLAP 引擎。
func Current() Engine { return current }

// UseClickHouse 是否以 ClickHouse 为 OLAP 引擎(否则 Doris)。
func UseClickHouse() bool { return current == ClickHouse }

// Set 强制指定引擎(仅测试使用)。
func Set(e Engine) { current = normalize(string(e)) }

// load 按优先级决定引擎。
func load() Engine { return loadWith(os.Getenv(EnvEngine), configDirs()) }

// loadWith 环境变量优先于配置文件。
func loadWith(envVal string, dirs []string) Engine {
	if v := strings.TrimSpace(envVal); v != "" {
		return normalize(v)
	}
	return loadFrom(dirs)
}

func loadFrom(dirs []string) Engine {
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, ConfigFile))
		if err != nil {
			continue
		}
		var cfg struct {
			Olap struct {
				Engine string `yaml:"engine"`
			} `yaml:"olap"`
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			warnf("parse %s: %v", filepath.Join(dir, ConfigFile), err)
			continue
		}
		if cfg.Olap.Engine != "" {
			return normalize(cfg.Olap.Engine)
		}
	}
	return Doris
}

// configDirs 配置目录候选:UBA_CONFIGS(路径列表)优先,否则 ./configs。
func configDirs() []string {
	if v := os.Getenv("UBA_CONFIGS"); v != "" {
		var dirs []string
		for _, part := range strings.Split(v, string(os.PathListSeparator)) {
			dirs = append(dirs, strings.Split(part, ",")...)
		}
		return dirs
	}
	return []string{"configs"}
}

// normalize 归一引擎名;未知值回退 Doris 并告警(fail-safe)。
func normalize(v string) Engine {
	switch e := Engine(strings.ToLower(strings.TrimSpace(string(v)))); e {
	case Doris, ClickHouse:
		return e
	default:
		warnf("unknown engine %q, fallback to %s", v, Doris)
		return Doris
	}
}

// warnf init 期只依赖 stderr,不引日志栈。
func warnf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[olapcfg] "+format+"\n", args...)
}
