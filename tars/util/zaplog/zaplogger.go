package zaplog

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	rotate "github.com/lestrrat-go/file-rotatelogs"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	zapLogger *zap.Logger
)

func InitZapLogger(options ...zapLoggerOption) error {
	var (
		zapLoggerLevel zap.AtomicLevel
		err            error
	)
	config := defaultOptions
	for _, option := range options {
		option.apply(&config)
	}

	if zapLogger, zapLoggerLevel, err = zapLoggerInit(&config); err != nil {
		fmt.Printf("ZapLogInit err: %v", err)
		return err
	}

	zapLogger = zapLogger.WithOptions(zap.AddCallerSkip(1))
	runZapLoggerHttpServer(&config, zapLoggerLevel)
	return nil
}

func zapLoggerInit(config *zapLoggerConf) (*zap.Logger, zap.AtomicLevel, error) {
	var (
		zapLogger      *zap.Logger
		zapLoggerLevel zap.AtomicLevel
		err            error
	)

	zapEncoderConfig := zap.NewProductionEncoderConfig()
	zapEncoderConfig.TimeKey = "timestamp"
	zapEncoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format("2006-01-02 15:04:05.000"))
	}
	zapEncoder := zapcore.NewJSONEncoder(zapEncoderConfig)
	zapWriter, err := getWriter(config.logPath)
	if err != nil {
		fmt.Printf("zapLoggerInit err: %v", err)
		return zapLogger, zapLoggerLevel, err
	}
	if config.isTestEnv {
		zapLoggerLevel = zap.NewAtomicLevelAt(zap.DebugLevel)
	} else {
		zapLoggerLevel = zap.NewAtomicLevelAt(zap.InfoLevel)
	}
	zapLogger = zap.New(zapcore.NewCore(zapEncoder, zapcore.AddSync(zapWriter), zapLoggerLevel),
		zap.AddCaller(), zap.AddStacktrace(zap.DPanicLevel))

	if config.withPid {
		zapLogger = zapLogger.With(zap.Int("pid", os.Getpid()))
	}
	if config.hostName != "" {
		zapLogger = zapLogger.With(zap.String("hostname", config.hostName))
	}
	if config.eLKTempName != "" {
		zapLogger = zapLogger.With(zap.String("service", config.eLKTempName))
	}

	return zapLogger, zapLoggerLevel, nil
}

func getWriter(filename string) (io.Writer, error) {
	hook, err := rotate.New(
		filename+".%Y%m%d%H", // 没有使用go风格
		rotate.WithLinkName(filename),
		rotate.WithMaxAge(time.Hour*24*3),     // 保存3天
		rotate.WithRotationTime(time.Hour*24), // 切割频率:24小时
	)
	if err != nil {
		fmt.Printf("getWriter err: %v", err)
		return hook, err
	}

	return hook, nil
}

// Debug logs a message at DebugLevel. The message includes any fields passed
// at the log site, as well as any fields accumulated on the logger.
func Debug(msg string, fields ...zapcore.Field) {
	zapLogger.Debug(msg, fields...)
}

// Info logs a message at InfoLevel. The message includes any fields passed
// at the log site, as well as any fields accumulated on the logger.
func Info(msg string, fields ...zapcore.Field) {
	zapLogger.Info(msg, fields...)
}

// Warn logs a message at WarnLevel. The message includes any fields passed
// at the log site, as well as any fields accumulated on the logger.
func Warn(msg string, fields ...zapcore.Field) {
	zapLogger.Warn(msg, fields...)
}

// Error logs a message at ErrorLevel. The message includes any fields passed
// at the log site, as well as any fields accumulated on the logger.
func Error(msg string, fields ...zapcore.Field) {
	zapLogger.Error(msg, fields...)
}

// DPanic logs a message at DPanicLevel. The message includes any fields passed
// at the log site, as well as any fields accumulated on the logger.
//
// If the logger is in development mode, it then panics (DPanic means
// "development panic"). This is useful for catching errors that are
// recoverable, but shouldn't ever happen.
func DPanic(msg string, fields ...zapcore.Field) {
	zapLogger.DPanic(msg, fields...)
}

// Panic logs a message at PanicLevel. The message includes any fields passed
// at the log site, as well as any fields accumulated on the logger.
//
// The logger then panics, even if logging at PanicLevel is disabled.
func Panic(msg string, fields ...zapcore.Field) {
	zapLogger.Panic(msg, fields...)
}

// Fatal logs a message at FatalLevel. The message includes any fields passed
// at the log site, as well as any fields accumulated on the logger.
//
// The logger then calls os.Exit(1), even if logging at FatalLevel is disabled.
func Fatal(msg string, fields ...zapcore.Field) {
	zapLogger.Fatal(msg, fields...)
}

func Sync() error {
	return zapLogger.Sync()
}

func SetLogLevel(level string) error {
	switch strings.ToLower(level) {
	case "debug", "info", "warn", "error", "panic", "fatal":
		level = strings.ToLower(level)
	case "all":
		level = "debug"
	case "none":
		level = "fatal"
	default:
		return errors.New("not support level")
	}

	client := http.Client{}
	type PayLoad struct {
		Level string `json:"level"`
	}
	data, err := json.Marshal(PayLoad{
		Level: level,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, zapLoggerHttpServer, bytes.NewReader(data))
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
