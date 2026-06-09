package log

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	ctrl "sigs.k8s.io/controller-runtime"
)

type Logger struct {
	logr.Logger
	// warnSugar is only used to produce log entries with WARNING level.
	warnSugar *zap.SugaredLogger
}

func (l *Logger) Error(err error, msg string, keysAndValues ...interface{}) {
	l.Logger.Error(err, msg, keysAndValues...)
}

func (l *Logger) Info(msg string, keysAndValues ...interface{}) {
	l.Logger.Info(msg, keysAndValues...)
}

func (l *Logger) Debug(msg string, keysAndValues ...interface{}) {
	l.Logger.V(1).Info(msg, keysAndValues...)
}

func (l *Logger) Warning(msg string, keysAndValues ...interface{}) {
	if l.warnSugar != nil {
		l.warnSugar.Warnw(msg, keysAndValues...)
		return
	}
	// default to logr.Logger if the underlying logger is not zapr.
	l.Logger.Error(nil, msg, keysAndValues...)
}

func GetLogger(ctx context.Context) Logger {
	logrLogger := ctrl.LoggerFrom(ctx)

	var warnSugar *zap.SugaredLogger
	if u, ok := logrLogger.GetSink().(zapr.Underlier); ok {
		warnSugar = u.GetUnderlying().
			WithOptions(
				zap.AddCallerSkip(1),
				zap.AddStacktrace(zapcore.ErrorLevel),
			).
			Sugar()
	}
	return Logger{
		Logger:    logrLogger,
		warnSugar: warnSugar,
	}
}
