package logger

import (
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

type LogLevel string

const (
	LevelFatal LogLevel = "fatal"
	LevelError LogLevel = "error"
	LevelWarn  LogLevel = "warn"
	LevelInfo  LogLevel = "info"
	LevelDebug LogLevel = "debug"
	LevelTrace LogLevel = "trace"
)

var validLevels = []LogLevel{LevelFatal, LevelError, LevelWarn, LevelInfo, LevelDebug, LevelTrace}

var (
	Log          zerolog.Logger
	currentLevel LogLevel
	mu           sync.RWMutex
)

type Config struct {
	Level       string
	Format      string
	Development bool
	DebugMode   bool
}

func isValidLevel(level string) bool {
	for _, l := range validLevels {
		if string(l) == level {
			return true
		}
	}
	return false
}

func getInitialLogLevel(cfg Config) LogLevel {
	envLevel := strings.ToLower(cfg.Level)
	if envLevel != "" && isValidLevel(envLevel) {
		return LogLevel(envLevel)
	}
	if cfg.DebugMode {
		return LevelDebug
	}
	if cfg.Development {
		return LevelDebug
	}
	return LevelInfo
}

func logLevelToZerolog(level LogLevel) zerolog.Level {
	switch level {
	case LevelTrace:
		return zerolog.TraceLevel
	case LevelDebug:
		return zerolog.DebugLevel
	case LevelInfo:
		return zerolog.InfoLevel
	case LevelWarn:
		return zerolog.WarnLevel
	case LevelError:
		return zerolog.ErrorLevel
	case LevelFatal:
		return zerolog.FatalLevel
	default:
		return zerolog.InfoLevel
	}
}

func Init(cfg Config) {
	initialLevel := getInitialLogLevel(cfg)
	mu.Lock()
	currentLevel = initialLevel
	mu.Unlock()

	zerolog.SetGlobalLevel(logLevelToZerolog(initialLevel))

	var writer io.Writer
	if cfg.Format == "text" || cfg.Development {
		writer = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: "2006-01-02 15:04:05",
			NoColor:    false,
			FormatLevel: func(i interface{}) string {
				if ll, ok := i.(string); ok {
					return strings.ToUpper(ll)
				}
				return "???"
			},
		}
	} else {
		writer = os.Stdout
	}

	zerolog.TimeFieldFormat = time.RFC3339

	Log = zerolog.New(writer).With().Timestamp().Logger()
}

func SetLogLevel(newLevel LogLevel) {
	if !isValidLevel(string(newLevel)) {
		return
	}
	mu.Lock()
	currentLevel = newLevel
	mu.Unlock()
	zerolog.SetGlobalLevel(logLevelToZerolog(newLevel))
	Log.Info().Msgf("Log level changed to: %s", newLevel)
}

func GetLogLevel() LogLevel {
	mu.RLock()
	defer mu.RUnlock()
	return currentLevel
}

func Trace() *zerolog.Event { return Log.Trace() }
func Debug() *zerolog.Event { return Log.Debug() }
func Info() *zerolog.Event  { return Log.Info() }
func Warn() *zerolog.Event  { return Log.Warn() }
func Error() *zerolog.Event { return Log.Error() }
func Fatal() *zerolog.Event { return Log.Fatal() }

func With() zerolog.Context {
	return Log.With()
}
