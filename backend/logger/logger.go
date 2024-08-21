package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var logger *zap.Logger

func init() {
	InitLogger()
}

func InitLogger() {
	config := zap.NewProductionConfig()
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	logger, _ = config.Build()
}

func NewLogger() *zap.Logger {
	return logger
}

func Debug(msg string, fields ...zap.Field) {
	logger.Debug(msg, fields...)
}

func Info(msg string, fields ...zap.Field) {
	logger.Info(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	logger.Warn(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	logger.Error(msg, fields...)
}

func Fatal(msg string, fields ...zap.Field) {
	logger.Fatal(msg, fields...)
}

func Sync() {
	logger.Sync()
}

func DebugEnable(enable bool) {
	if enable {
		logger = logger.WithOptions(zap.IncreaseLevel(zapcore.DebugLevel))
	} else {
		logger = logger.WithOptions(zap.IncreaseLevel(zapcore.InfoLevel))
	}
}

func SetOutput(w zapcore.WriteSyncer) {
	logger = logger.WithOptions(zap.WrapCore(func(core zapcore.Core) zapcore.Core {
		return zapcore.NewCore(
			zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
			w,
			zapcore.DebugLevel,
		)
	}))
}

func GetOutput() zapcore.WriteSyncer {
	return zapcore.Lock(os.Stdout)
}
