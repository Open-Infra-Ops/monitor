package PrometheusClient

import (
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/pkg/errors"
	"github.com/prometheus/common/promlog"
	"os"
)

var (
	// Application wide logger
	logger log.Logger
)

type AllowedLevel struct {
	s string
	o level.Option
}

// Set updates the value of the allowed level.
func (l *AllowedLevel) Set(s string) error {
	switch s {
	case "debug":
		l.o = level.AllowDebug()
	case "info":
		l.o = level.AllowInfo()
	case "warn":
		l.o = level.AllowWarn()
	case "error":
		l.o = level.AllowError()
	default:
		return errors.Errorf("unrecognized log level %q", s)
	}
	l.s = s
	return nil
}

func New(al AllowedLevel) log.Logger {
	l := log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout))
	l = level.NewFilter(l, al.o)
	l = log.With(l, "ts", log.DefaultTimestampUTC, "caller", log.DefaultCaller)
	return l
}

func Init(logLevel string) {
	allowedLevel := promlog.AllowedLevel{}
	allowedLevel.Set(logLevel)
	logConfig := promlog.Config{
		Level: &allowedLevel,
	}
	logger = promlog.New(&logConfig)
	logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stdout))
	logger = log.With(logger, "ts", log.DefaultTimestampUTC, "caller", log.DefaultCaller)
}

func Debug(keyValues ...interface{}) {
	level.Debug(logger).Log(keyValues...)
}

func Info(keyValues ...interface{}) {
	level.Info(logger).Log(keyValues...)
}

func Warn(keyValues ...interface{}) {
	level.Warn(logger).Log(keyValues...)
}

func Error(keyValues ...interface{}) {
	level.Error(logger).Log(keyValues...)
}
