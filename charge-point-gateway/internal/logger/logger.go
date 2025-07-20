package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/diode"
	"github.com/rs/zerolog/log"
)

// Logger 日志管理器
type Logger struct {
	logger  zerolog.Logger
	config  *Config
	logFile *os.File // 用于文件输出时的文件句柄
}

// Config 日志配置
type Config struct {
	Level      string `json:"level"`      // 日志级别: debug, info, warn, error
	Format     string `json:"format"`     // 输出格式: console, json
	Output     string `json:"output"`     // 输出目标: stdout, stderr, file path
	TimeFormat string `json:"timeFormat"` // 时间格式
	Caller     bool   `json:"caller"`     // 是否显示调用者信息
	Async      bool   `json:"async"`      // 是否启用异步日志
}

// DefaultConfig 默认日志配置
func DefaultConfig() *Config {
	return &Config{
		Level:      "info",
		Format:     "console",
		Output:     "stdout",
		TimeFormat: time.RFC3339,
		Caller:     true,
		Async:      false, // 默认同步，可通过配置启用异步
	}
}

// New 创建新的日志管理器
func New(config *Config) (*Logger, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// 设置全局时间格式
	zerolog.TimeFieldFormat = config.TimeFormat

	// 设置日志级别
	level, err := zerolog.ParseLevel(config.Level)
	if err != nil {
		return nil, fmt.Errorf("invalid log level %s: %w", config.Level, err)
	}
	zerolog.SetGlobalLevel(level)

	// 配置输出目标
	var output io.Writer
	switch strings.ToLower(config.Output) {
	case "stdout":
		output = os.Stdout
	case "stderr":
		output = os.Stderr
	default:
		// 文件输出
		if err := ensureDir(filepath.Dir(config.Output)); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}
		file, err := os.OpenFile(config.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file %s: %w", config.Output, err)
		}
		output = file
	}

	// 如果启用异步，使用diode包装输出
	if config.Async {
		// 使用zerolog官方推荐的diode异步writer
		// 参数：输出目标，缓冲区大小，刷新间隔，丢弃回调
		output = diode.NewWriter(output, 1000, 10*time.Millisecond, func(missed int) {
			// 当缓冲区满时的回调，记录丢弃的日志数量
			fmt.Fprintf(os.Stderr, "Logger dropped %d messages\n", missed)
		})
	}

	// 配置输出格式
	var logger zerolog.Logger
	switch strings.ToLower(config.Format) {
	case "console":
		logger = zerolog.New(zerolog.ConsoleWriter{
			Out:        output,
			TimeFormat: config.TimeFormat,
		})
	case "json":
		logger = zerolog.New(output)
	default:
		return nil, fmt.Errorf("unsupported log format: %s", config.Format)
	}

	// 添加时间戳
	logger = logger.With().Timestamp().Logger()

	// 添加调用者信息
	if config.Caller {
		logger = logger.With().Caller().Logger()
	}

	// 设置日志级别到具体的 logger 实例
	logger = logger.Level(level)

	// 设置为全局日志器 - 确保全局 zerolog 也使用相同的配置
	log.Logger = logger

	// 同时设置我们自己的全局 logger
	globalLogger = &Logger{
		logger: logger,
		config: config,
	}

	return &Logger{
		logger: logger,
		config: config,
	}, nil
}

// GetLogger 获取日志器实例
func (l *Logger) GetLogger() zerolog.Logger {
	return l.logger
}

// Debug 调试日志
func (l *Logger) Debug(msg string) {
	l.logger.Debug().Msg(msg)
}

// Debugf 格式化调试日志
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.logger.Debug().Msgf(format, args...)
}

// Info 信息日志
func (l *Logger) Info(msg string) {
	l.logger.Info().Msg(msg)
}

// Infof 格式化信息日志
func (l *Logger) Infof(format string, args ...interface{}) {
	l.logger.Info().Msgf(format, args...)
}

// Warn 警告日志
func (l *Logger) Warn(msg string) {
	l.logger.Warn().Msg(msg)
}

// Warnf 格式化警告日志
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.logger.Warn().Msgf(format, args...)
}

// Error 错误日志
func (l *Logger) Error(msg string) {
	l.logger.Error().Msg(msg)
}

// Errorf 格式化错误日志
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.logger.Error().Msgf(format, args...)
}

// ErrorWithErr 带错误对象的错误日志
func (l *Logger) ErrorWithErr(err error, msg string) {
	l.logger.Error().Err(err).Msg(msg)
}

// Fatal 致命错误日志
func (l *Logger) Fatal(msg string) {
	l.logger.Fatal().Msg(msg)
}

// Fatalf 格式化致命错误日志
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.logger.Fatal().Msgf(format, args...)
}

// WithField 添加字段
func (l *Logger) WithField(key string, value interface{}) *zerolog.Event {
	return l.logger.Info().Interface(key, value)
}

// WithFields 添加多个字段
func (l *Logger) WithFields(fields map[string]interface{}) *zerolog.Event {
	event := l.logger.Info()
	for k, v := range fields {
		event = event.Interface(k, v)
	}
	return event
}

// SetLevel 动态设置日志级别
func (l *Logger) SetLevel(level string) error {
	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		return fmt.Errorf("invalid log level %s: %w", level, err)
	}

	l.logger = l.logger.Level(lvl)
	l.config.Level = level
	return nil
}

// GetLevel 获取当前日志级别
func (l *Logger) GetLevel() string {
	return l.config.Level
}

// Close 关闭日志器 (实际上zerolog不需要显式关闭)
// 这个方法主要是为了接口完整性，在大多数情况下不需要调用
func (l *Logger) Close() error {
	// zerolog会自动处理文件刷新和关闭
	// 这里保留接口是为了未来可能的扩展需求
	return nil
}

// ensureDir 确保目录存在
func ensureDir(dir string) error {
	if dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0755)
}

// 全局便捷函数
var globalLogger *Logger

// InitGlobalLogger 初始化全局日志器
func InitGlobalLogger(config *Config) error {
	logger, err := New(config)
	if err != nil {
		return err
	}
	globalLogger = logger
	return nil
}

// Debug 全局调试日志
func Debug(msg string) {
	if globalLogger != nil {
		globalLogger.Debug(msg)
	}
}

// Debugf 全局格式化调试日志
func Debugf(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Debugf(format, args...)
	}
}

// Info 全局信息日志
func Info(msg string) {
	if globalLogger != nil {
		globalLogger.Info(msg)
	}
}

// Infof 全局格式化信息日志
func Infof(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Infof(format, args...)
	}
}

// Warn 全局警告日志
func Warn(msg string) {
	if globalLogger != nil {
		globalLogger.Warn(msg)
	}
}

// Warnf 全局格式化警告日志
func Warnf(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Warnf(format, args...)
	}
}

// Error 全局错误日志
func Error(msg string) {
	if globalLogger != nil {
		globalLogger.Error(msg)
	}
}

// Errorf 全局格式化错误日志
func Errorf(format string, args ...interface{}) {
	if globalLogger != nil {
		globalLogger.Errorf(format, args...)
	}
}

// ErrorWithErr 全局带错误对象的错误日志
func ErrorWithErr(err error, msg string) {
	if globalLogger != nil {
		globalLogger.ErrorWithErr(err, msg)
	}
}
