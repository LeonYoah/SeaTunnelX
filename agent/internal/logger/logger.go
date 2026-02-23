package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/seatunnel/seatunnelX/agent/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	rootLogger *zap.SugaredLogger
	initOnce   sync.Once
	initErr    error
)

// Init 初始化 Agent 日志：
// - 同时输出到控制台和日志文件
// - 日志文件路径使用 cfg.Log.File（默认 /var/log/seatunnelx-agent/agent.log）
func Init(cfg *config.Config) error {
	initOnce.Do(func() {
		if cfg == nil {
			initErr = fmt.Errorf("nil config")
			return
		}

		w, err := buildWriter(cfg)
		if err != nil {
			initErr = err
			return
		}

		encoderCfg := zapcore.EncoderConfig{
			TimeKey:        "time",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "msg",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		}

		level := parseLevel(cfg.Log.Level)

		core := zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderCfg),
			w,
			level,
		)

		z := zap.New(core,
			zap.AddCaller(),
			zap.AddCallerSkip(1),
		)
		rootLogger = z.Sugar()
	})

	return initErr
}

func buildWriter(cfg *config.Config) (zapcore.WriteSyncer, error) {
	logPath := cfg.Log.File
	if logPath == "" {
		logPath = config.DefaultLogFile
	}

	logDir := filepath.Dir(logPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("[AgentLogger] create log dir err: %w", err)
	}

	fileWriter := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    cfg.Log.MaxSize,
		MaxBackups: cfg.Log.MaxBackups,
		MaxAge:     cfg.Log.MaxAge,
		Compress:   false,
	}

	console := zapcore.AddSync(os.Stdout)
	file := zapcore.AddSync(fileWriter)

	return zapcore.NewMultiWriteSyncer(console, file), nil
}

func parseLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "info", "":
		return zapcore.InfoLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "dpanic":
		return zapcore.DPanicLevel
	case "panic":
		return zapcore.PanicLevel
	case "fatal":
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}

// L 返回底层 *zap.SugaredLogger，便于在复杂场景下直接使用
func L() *zap.SugaredLogger {
	if rootLogger == nil {
		// 后备一个简易 logger，避免 nil 崩溃
		z, _ := zap.NewDevelopment()
		return z.Sugar()
	}
	return rootLogger
}

func Debugf(format string, args ...interface{}) {
	L().Debugf(format, args...)
}

func Infof(format string, args ...interface{}) {
	L().Infof(format, args...)
}

func Warnf(format string, args ...interface{}) {
	L().Warnf(format, args...)
}

func Errorf(format string, args ...interface{}) {
	L().Errorf(format, args...)
}

