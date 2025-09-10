package log

type Logger interface {
	Debug() *LogEvent
	Info() *LogEvent
	Warn() *LogEvent
	Error() *LogEvent
	Fatal() *LogEvent
	IgnoreCheckLevel() bool
	GetAppender() []LogAppender
	AddAppender(appender LogAppender)
	OnEventEnd(e *LogEvent)
}

var _defaultLogger *GameLogger

func init() {
	_defaultLogger = NewLogger(nil)
}

// AddAppender adds a new log appender to the default logger.
// This is a convenience function for the package-level default logger.
func AddAppender(appender LogAppender) {
	_defaultLogger.AddAppender(appender)
}

// Refresh triggers a refresh operation on all appenders of the default logger.
// This is a convenience function for the package-level default logger.
func Refresh() {
	_defaultLogger.Refresh()
}

// SetDefaultLogger replaces the default logger with a custom instance.
// This allows global configuration of the package-level logging functions.
func SetDefaultLogger(logger *GameLogger) {
	_defaultLogger = logger
}

// Debug creates a new debug-level log event using the default logger.
// This is a convenience function for the package-level default logger.
func Debug() *LogEvent {
	return _defaultLogger.Debug()
}

// Info creates a new info-level log event using the default logger.
// This is a convenience function for the package-level default logger.
func Info() *LogEvent {
	return _defaultLogger.Info()
}

// Warn creates a new warning-level log event using the default logger.
// This is a convenience function for the package-level default logger.
func Warn() *LogEvent {
	return _defaultLogger.Warn()
}

// Error creates a new error-level log event using the default logger.
// This is a convenience function for the package-level default logger.
func Error() *LogEvent {
	return _defaultLogger.Error()
}

// Fatal creates a new fatal-level log event using the default logger.
// This is a convenience function for the package-level default logger.
// After logging, the application will terminate.
func Fatal() *LogEvent {
	return _defaultLogger.Fatal()
}
