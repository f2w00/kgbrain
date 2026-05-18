package config

import "time"

type Config struct {
	Server ServerConfig `mapstructure:"server"`
	Log    LogConfig    `mapstructure:"log"`
	Store  StoreConfig  `mapstructure:"store"`
}

type ServerConfig struct {
	Address      string        `mapstructure:"address"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

type LogConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	Output     string `mapstructure:"output"`
	TimeFormat string `mapstructure:"time_format"`
}

type StoreConfig struct {
	Path    string   `mapstructure:"path"`
	Pragmas []string `mapstructure:"pragmas"`
}

func Default() Config {
	return Config{
		Server: ServerConfig{
			Address:     ":8080",
			ReadTimeout: 30 * time.Second,
			WriteTimeout: 60 * time.Second,
		},
		Log: LogConfig{
			Level:      "info",
			Format:     "json",
			Output:     "stdout",
			TimeFormat: "2006-01-02T15:04:05Z0700",
		},
		Store: StoreConfig{
			Path: "data/kgbrain.db",
			Pragmas: []string{
				"PRAGMA journal_mode = WAL",
				"PRAGMA synchronous = NORMAL",
				"PRAGMA busy_timeout = 5000",
			},
		},
	}
}
