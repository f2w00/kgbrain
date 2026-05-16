package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

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
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("read config file: %w", err)
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
