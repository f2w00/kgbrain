package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"kgbrain/internal/config"
)

var global *zap.Logger

func Init(cfg config.LogConfig) error {
	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		level = zapcore.InfoLevel
	}

	encCfg := zap.NewProductionEncoderConfig()
	encCfg.TimeKey = "ts"
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	var encoder zapcore.Encoder
	if cfg.Format == "console" {
		encoder = zapcore.NewConsoleEncoder(encCfg)
	} else {
		encoder = zapcore.NewJSONEncoder(encCfg)
	}

	var writer zapcore.WriteSyncer
	switch cfg.Output {
	case "stderr":
		writer = zapcore.Lock(os.Stderr)
	default:
		writer = zapcore.Lock(os.Stdout)
	}

	if cfg.TimeFormat != "" {
		encCfg.EncodeTime = zapcore.TimeEncoderOfLayout(cfg.TimeFormat)
	}

	core := zapcore.NewCore(encoder, writer, level)
	global = zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))

	zap.ReplaceGlobals(global)
	return nil
}

func L() *zap.Logger {
	if global == nil {
		global = zap.NewNop()
	}
	return global
}
