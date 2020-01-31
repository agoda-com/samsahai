package logger

import (
	"fmt"

	"github.com/go-logr/logr"
	"go.uber.org/zap"
	"sigs.k8s.io/controller-runtime/pkg/log"
	crzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/agoda-com/samsahai/internal"
)

// Log is the base logger used by kubebuilder.  It delegates
// to another logr.Logger.  You *must* call SetLogger to
// get any actual logging.
var Log = NewS2hLogger(log.NullLogger{})

var S2HLog Logger

func init() {
	S2HLog = Log.WithName(internal.AppName)
}

// DelegatingLogger is a logr.Logger that delegates to another logr.Logger.
// If the underlying promise is not nil, it registers calls to sub-loggers with
// the logging factory to be populated later, and returns a new delegating
// logger.  It expects to have *some* logr.Logger set at all times (generally
// a no-op logger before the promises are fulfilled).
type DelegatingLogger struct {
	logger *log.DelegatingLogger
}

func GetLogger(debug bool) logr.Logger {
	l := crzap.New(func(o *crzap.Options) {
		o.Development = debug
		lvl := zap.NewAtomicLevelAt(zap.ErrorLevel)
		o.StacktraceLevel = &lvl
	})
	return l
}

func NewS2hLogger(initial logr.Logger) *DelegatingLogger {
	return &DelegatingLogger{
		logger: log.NewDelegatingLogger(initial),
	}
}

// SetLogger sets a concrete logging implementation for all deferred Loggers.
func SetLogger(l logr.Logger) {
	Log.Fulfill(l)
}

// Fulfill switches the logger over to use the actual logger
// provided, instead of the temporary initial one, if this method
// has not been previously called.
func (l *DelegatingLogger) Fulfill(actual logr.Logger) {
	l.logger.Fulfill(actual)
}

// Logger represents the ability to log messages, both errors and not.
type Logger interface {
	// All Loggers implement InfoLogger.  Calling InfoLogger methods directly on
	// a Logger value is equivalent to calling them on a V(0) InfoLogger.  For
	// example, logger.Info() produces the same result as logger.V(0).Info.
	logr.InfoLogger

	// Error logs an error, with the given message and key/value pairs as context.
	// It functions similarly to calling Info with the "error" named value, but may
	// have unique behavior, and should be preferred for logging errors (see the
	// package documentations for more information).
	//
	// The msg field should be used to add context to any underlying error,
	// while the err field should be used to attach the actual error that
	// triggered this log line, if present.
	Error(err error, msg string, keysAndValues ...interface{})

	// Debug logs a non-error message with the given key/value pairs as context.
	// It functions similarly to calling Info with verbosity level as 1.
	//
	// The msg argument should be used to add some constant description to
	// the log line. The key/value pairs can then be used to add additional
	// variable information.  The key/value pairs should alternate string
	// keys and arbitrary values.
	Debug(msg string, keysAndValues ...interface{})

	// Warn logs a non-error message with the given key/value pairs as context.
	// It functions similarly to calling Info with verbosity level as -1.
	//
	// The msg argument should be used to add some constant description to
	// the log line. The key/value pairs can then be used to add additional
	// variable information.  The key/value pairs should alternate string
	// keys and arbitrary values.
	Warn(msg string, keysAndValues ...interface{})

	Warnf(format string, args ...interface{})

	// WithValues adds some key-value pairs of context to a logger.
	// See Info for documentation on how key/value pairs work.
	WithValues(keysAndValues ...interface{}) Logger

	// WithName adds a new element to the logger's name.
	// Successive calls with WithName continue to append
	// suffixes to the logger's name.  It's strongly reccomended
	// that name segments contain only letters, digits, and hyphens
	// (see the package documentation for more information).
	WithName(name string) Logger
}

// Enabled implements logr.InfoLogger
func (l *DelegatingLogger) Enabled() bool {
	return l.logger.Enabled()
}

// Info implements logr.InfoLogger
func (l *DelegatingLogger) Info(msg string, keyAndValues ...interface{}) {
	l.logger.Info(msg, keyAndValues...)
}

// Error implements logr.Logger
func (l *DelegatingLogger) Error(err error, msg string, keyAndValues ...interface{}) {
	l.logger.Error(err, msg, keyAndValues...)
}

// Debug implements logr.Logger
func (l *DelegatingLogger) Debug(msg string, keysAndValues ...interface{}) {
	l.logger.V(1).Info(msg, keysAndValues...)
}

// Warn implements logr.Logger
func (l *DelegatingLogger) Warn(msg string, keysAndValues ...interface{}) {
	l.logger.V(-1).Info(msg, keysAndValues...)
}

// Warnf prints a message with the specified format
func (l *DelegatingLogger) Warnf(format string, args ...interface{}) {
	l.logger.V(-1).Info(fmt.Sprintf(format, args...))
}

// WithName implements logr.Logger
func (l *DelegatingLogger) WithName(name string) Logger {
	ln := l.logger.WithName(name)
	delegatingLn, ok := ln.(*log.DelegatingLogger)

	if !ok {
		delegatingLn = log.NewDelegatingLogger(log.NullLogger{})
	}

	res := &DelegatingLogger{
		logger: delegatingLn,
	}

	return res
}

// WithValues implements logr.Logger
func (l *DelegatingLogger) WithValues(tags ...interface{}) Logger {
	ln := l.logger.WithValues(tags)
	delegatingLn, ok := ln.(*log.DelegatingLogger)
	if !ok {
		delegatingLn = log.NewDelegatingLogger(log.NullLogger{})
	}

	res := &DelegatingLogger{
		logger: delegatingLn,
	}

	return res
}
