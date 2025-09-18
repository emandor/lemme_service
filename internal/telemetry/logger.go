package telemetry

import (
	"io"
	"os"
	"strconv"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Config struct {
	Level      string
	JSON       bool
	File       string
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	Compress   bool
}

var log zerolog.Logger

func Init(cfg Config) zerolog.Logger {
	zerolog.TimeFieldFormat = time.RFC3339
	// file rotator
	rotator := &lumberjack.Logger{
		Filename:   cfg.File,
		MaxSize:    ifZero(cfg.MaxSizeMB, 10),
		MaxBackups: ifZero(cfg.MaxBackups, 3),
		MaxAge:     ifZero(cfg.MaxAgeDays, 28),
		Compress:   cfg.Compress,
	}

	var console io.Writer = os.Stdout

	if !cfg.JSON {
		console = zerolog.NewConsoleWriter(func(w *zerolog.ConsoleWriter) {
			w.TimeFormat = time.RFC3339
		})
	}
	multi := zerolog.MultiLevelWriter(console, rotator)

	l := zerolog.New(multi).With().Timestamp().Logger()

	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	l = l.Level(level)

	log = l
	return log
}

func L() zerolog.Logger { return log }

func FromEnv(get func(string, string) string) Config {
	return Config{
		Level:      get("LOG_LEVEL", "info"),
		JSON:       parseBool(get("LOG_JSON", "true")),
		File:       get("LOG_FILE", "app.log"),
		MaxSizeMB:  atoi(get("LOG_MAX_SIZE_MB", "10")),
		MaxBackups: atoi(get("LOG_MAX_BACKUPS", "3")),
		MaxAgeDays: atoi(get("LOG_MAX_AGE_DAYS", "28")),
		Compress:   parseBool(get("LOG_COMPRESS", "true")),
	}
}

func ifZero[T ~int](v T, d T) T {
	if v == 0 {
		return d
	}
	return v
}
func atoi(s string) int       { i, _ := strconv.Atoi(s); return i }
func parseBool(s string) bool { b, _ := strconv.ParseBool(s); return b }
