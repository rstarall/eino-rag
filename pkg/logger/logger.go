package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var log *zap.Logger

// Init 初始化日志
func Init(mode string) error {
	var config zap.Config

	if mode == "release" {
		config = zap.NewProductionConfig()
		config.EncoderConfig.TimeKey = "timestamp"
		config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	} else {
		config = zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	// 同时输出到文件和控制台
	config.OutputPaths = []string{"stdout", "logs/app.log"}
	config.ErrorOutputPaths = []string{"stderr", "logs/error.log"}

	// 确保日志目录存在
	if err := os.MkdirAll("logs", 0755); err != nil {
		return err
	}

	var err error
	log, err = config.Build()
	if err != nil {
		return err
	}

	return nil
}

// Get 获取日志实例
func Get() *zap.Logger {
	if log == nil {
		// 如果未初始化，使用默认配置
		log, _ = zap.NewProduction()
	}
	return log
}

// Sync 同步日志缓冲
func Sync() error {
	if log != nil {
		return log.Sync()
	}
	return nil
}