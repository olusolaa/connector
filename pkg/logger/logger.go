package logger

import (
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Blue   = "\033[34m"
	White  = "\033[37m"
)

func Colorize(color, text string) string {
	return color + text + Reset
}

func newConsoleWriter() zerolog.ConsoleWriter {
	cw := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	}

	cw.FormatLevel = func(i interface{}) string {
		if i == nil {
			return ""
		}
		s, ok := i.(string)
		if !ok {
			return ""
		}
		switch strings.ToLower(s) {
		case "debug":
			return Colorize(Blue, strings.ToUpper(s)) + ":"
		case "info":
			return Colorize(Green, strings.ToUpper(s)) + ":"
		case "warn":
			return Colorize(Yellow, strings.ToUpper(s)) + ":"
		case "error":
			return Colorize(Red, strings.ToUpper(s)) + ":"
		case "fatal":
			return Colorize(Red, strings.ToUpper(s)) + ":"
		default:
			return Colorize(White, strings.ToUpper(s)) + ":"
		}
	}

	// Similarly, you can modify how the message is displayed:
	cw.FormatMessage = func(i interface{}) string {
		if i == nil {
			return ""
		}
		return Colorize(White, i.(string))
	}

	return cw
}

// baseLogger is the main logger you’ll use throughout your app.
var baseLogger = zerolog.New(newConsoleWriter()).
	With().
	Timestamp().
	Logger()

// For convenience, you can wrap zerolog methods:

func Info() *zerolog.Event {
	return baseLogger.Info()
}

func Warn() *zerolog.Event {
	return baseLogger.Warn()
}

func Error() *zerolog.Event {
	return baseLogger.Error()
}

func Fatal() *zerolog.Event {
	return baseLogger.Fatal()
}

func WithError(err error) *zerolog.Event {
	return baseLogger.Error().Err(err)
}

func WithFields(fields map[string]interface{}) *zerolog.Event {
	event := baseLogger.Info()
	for k, v := range fields {
		event = event.Interface(k, v)
	}
	return event
}
