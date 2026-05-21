package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

const defaultConfigContent = `[server]
address      = ":8848"
read_timeout  = "30s"
write_timeout = "60s"

# ---------- 日志 ----------
[log]
level       = "info"
format      = "json"
output      = "stdout"
time_format = "2006-01-02T15:04:05Z0700"

# ---------- 存储 ----------
[store]
path = "data/kgbrain.db"
pragmas = [
    "PRAGMA journal_mode = WAL",
    "PRAGMA synchronous = NORMAL",
    "PRAGMA busy_timeout = 5000",
]
`

func Load(cfg *Config, path string) error {
	v := viper.New()
	v.SetConfigType("toml")
	v.SetConfigFile(path)

	v.SetEnvPrefix("KGBRAIN")
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	d := Default()
	_ = v.Unmarshal(&d)

	for key, val := range flatten(d) {
		v.SetDefault(key, val)
	}

	if err := v.ReadInConfig(); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("read config file: %w", err)
		}
		// 配置文件不存在时，写入默认配置
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return fmt.Errorf("create config directory: %w", err)
		}
		if err := os.WriteFile(path, []byte(defaultConfigContent), 0644); err != nil {
			return fmt.Errorf("write default config: %w", err)
		}
		fmt.Printf("config file not found, created default: %s\n", path)
		// 重新读取刚创建的配置文件
		if err := v.ReadInConfig(); err != nil {
			return fmt.Errorf("read created config file: %w", err)
		}
	}

	return v.Unmarshal(cfg)
}

func flatten(v any) map[string]any {
	out := make(map[string]any)
	flattenMap("", v, out)
	return out
}

func flattenMap(prefix string, v any, out map[string]any) {
	switch m := v.(type) {
	case map[string]any:
		for k, vv := range m {
			key := k
			if prefix != "" {
				key = prefix + "." + k
			}
			flattenMap(key, vv, out)
		}
	default:
		out[prefix] = v
	}
}
