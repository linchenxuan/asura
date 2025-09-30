package log

import (
	"github.com/lcx/asura/config"
)

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

// SetDefaultLoggerWithConfigManager replaces the default logger with a custom instance
// that supports configuration manager for hot-reload functionality.
// This enables dynamic configuration changes for package-level logging functions.
//
// Parameters:
//   - logger: GameLogger instance with configuration manager support
//   - configManager: Configuration manager instance for hot-reload
func SetDefaultLoggerWithConfigManager(logger *GameLogger, configManager config.ConfigManager) {
	_defaultLogger = logger
}

// InitializeWithConfigManager initializes the default logger with configuration manager support.
// This function loads the logger configuration from the config manager and enables
// hot-reload functionality for package-level logging functions.
//
// Parameters:
//   - configManager: Configuration manager instance
//
// Returns:
//   - Error if configuration loading fails, nil otherwise
func InitializeWithConfigManager(configManager config.ConfigManager) error {
	if configManager == nil {
		return nil
	}

	// Load logger configuration
	logCfg := &LogCfg{}
	if err := configManager.LoadConfig("logger", logCfg); err != nil {
		return err
	}

	// Create logger with configuration manager support
	logger := NewLoggerWithConfigManager(logCfg, configManager)
	SetDefaultLoggerWithConfigManager(logger, configManager)

	return nil
}

// Initialize initializes the default logger using the singleton ConfigManager instance.
// This provides a simplified initialization method that uses the global configuration manager.
//
// Returns:
//   - Error if configuration loading fails, nil otherwise
func Initialize() error {
	configManager := config.GetInstance()
	return InitializeWithConfigManager(configManager)
}

// GetConfigManager returns the current configuration manager instance.
// This can be used to access configuration manager functionality from
// package-level logging functions.
//
// Returns:
//   - Current configuration manager instance, or nil if not set
func GetConfigManager() config.ConfigManager {
	return config.GetInstance()
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

// Warn creates a new warn-level log event using the default logger.
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
func Fatal() *LogEvent {
	return _defaultLogger.Fatal()
}
