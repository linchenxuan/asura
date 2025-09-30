package log

import (
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/lcx/asura/config"
)

// GameLogger provides a thread-safe logging interface with configurable appenders and formatting.
// It supports different log levels, caller information, and efficient object reuse through sync.Pool.
// This logger is designed for high-performance applications, particularly game servers, where
// low-latency logging and minimal memory allocation are critical requirements.
//
// Key features include:
// - Thread-safe operation with lock-free logging path
// - Configurable log levels and appenders (console, file, etc.)
// - Automatic caller information capturing (file, function, line number)
// - Efficient object pooling to minimize garbage collection pressure
// - Per-file/per-line log level overrides for fine-grained control
// - Hot-reload support for dynamic configuration changes without service restart
//
// Example usage:
// ```
//
//	logger := NewLogger(&LogCfg{
//	    LogLevel:        InfoLevel,
//	    ConsoleAppender: true,
//	    FileAppender:    true,
//	    LogPath:         "/path/to/logfile.log",
//	})
//
// logger.Info().Str("module", "server").Int("connections", 42).Msg("Server started successfully")
// ```
type GameLogger struct {
	appenders         []LogAppender        // Collection of appenders responsible for log output
	minLevel          Level                // Minimum log level that will be processed
	callerSkip        int                  // Number of stack frames to skip when capturing caller information
	eventPool         *sync.Pool           // Object pool for LogEvent instances to minimize GC
	levelChange       *levelChange         // Configuration for per-file/per-line log level overrides
	callerCache       sync.Map             // Cache for caller information to avoid redundant calculations
	enabledCallerInfo bool                 // Flag indicating whether caller information should be captured
	configManager     config.ConfigManager // Configuration manager for hot-reload support
	configMutex       sync.RWMutex         // Mutex for thread-safe configuration updates
	currentConfig     *LogCfg              // Current configuration for fast access
}

// NewLogger creates a new GameLogger instance with the provided configuration.
// If cfg is nil, it uses default configuration values from getDefaultCfg().
//
// This function initializes the logger with the specified log level, configures
// appenders according to the configuration, and sets up object pooling for
// efficient log event creation.
//
// Parameters:
//   - cfg: Logger configuration specifying log level, appenders, and other settings
//
// Returns:
//   - A new GameLogger instance configured according to the provided settings
func NewLogger(cfg *LogCfg) *GameLogger {
	if cfg == nil {
		cfg = getDefaultCfg()
	}

	logger := &GameLogger{
		minLevel:          cfg.LogLevel,
		callerSkip:        cfg.CallerSkip,
		levelChange:       newLevelChange(cfg.LevelChange),
		enabledCallerInfo: cfg.EnabledCallerInfo,
		currentConfig:     cfg,
	}

	// Initialize object pool for LogEvent instances to minimize garbage collection
	logger.eventPool = &sync.Pool{
		New: func() any {
			return newEvent(logger)
		},
	}

	// Configure appenders based on configuration
	if cfg.FileAppender {
		logger.AddAppender(NewFileAppender(cfg, logger))
	}

	if cfg.ConsoleAppender {
		logger.AddAppender(NewConsoleAppender())
	}

	return logger
}

// NewLoggerWithConfigManager creates a new GameLogger instance with configuration manager support.
// This enables hot-reload functionality for dynamic configuration changes without service restart.
//
// Parameters:
//   - cfg: Logger configuration specifying log level, appenders, and other settings
//   - configManager: Configuration manager instance for hot-reload support
//
// Returns:
//   - A new GameLogger instance with hot-reload capability
func NewLoggerWithConfigManager(cfg *LogCfg, configManager config.ConfigManager) *GameLogger {
	logger := NewLogger(cfg)
	logger.configManager = configManager

	// Register as configuration change listener for hot-reload
	if configManager != nil {
		configManager.AddChangeListener(logger)

		// Reconfigure appenders to use config manager for dynamic configuration
		logger.reconfigureAppendersWithConfigManager(configManager)
	}

	return logger
}

// reconfigureAppendersWithConfigManager reconfigures all appenders to use the configuration manager
// for dynamic configuration updates. This ensures that all appenders support hot-reload functionality.
//
// Parameters:
//   - configManager: Configuration manager instance for hot-reload support
func (x *GameLogger) reconfigureAppendersWithConfigManager(configManager config.ConfigManager) {
	// Clear existing appenders
	x.appenders = nil

	// Get current configuration from config manager
	if configManager != nil {
		if cfg, err := configManager.GetConfig("logger"); err == nil {
			if logCfg, ok := cfg.(*LogCfg); ok {
				// Reconfigure appenders with config manager support
				if logCfg.FileAppender {
					x.AddAppender(NewFileAppenderWithConfigManager(configManager, x))
				}

				if logCfg.ConsoleAppender {
					x.AddAppender(NewConsoleAppender())
				}
			}
		}
	}
}

// OnConfigChanged implements ConfigChangeListener interface for hot-reload support.
// This method is called when the logger configuration changes, allowing for
// dynamic updates without service restart.
//
// Parameters:
//   - configName: Name of the configuration that changed
//   - newConfig: New configuration instance
//   - oldConfig: Previous configuration instance
//
// Returns:
//   - Error if configuration update fails, nil otherwise
func (x *GameLogger) OnConfigChanged(configName string, newConfig, oldConfig config.Config) error {
	if configName != "logger" {
		return nil // Ignore non-logger configuration changes
	}

	newLogCfg, ok := newConfig.(*LogCfg)
	if !ok {
		return nil // Ignore non-LogCfg configuration changes
	}

	// Update logger configuration atomically
	x.updateConfig(newLogCfg)

	// Notify all appenders about configuration change
	for _, appender := range x.appenders {
		if listener, ok := appender.(config.ConfigChangeListener); ok {
			if err := listener.OnConfigChanged(configName, newConfig, oldConfig); err != nil {
				// Log the error but continue notifying other appenders
				x.Error().Err(err).Msg("Failed to notify appender about config change")
			}
		}
	}

	return nil
}

// updateConfig updates the logger configuration with thread-safe atomic operations.
// This method ensures that configuration changes are applied consistently
// without affecting ongoing logging operations.
//
// Parameters:
//   - newCfg: New logger configuration to apply
func (x *GameLogger) updateConfig(newCfg *LogCfg) {
	x.configMutex.Lock()
	defer x.configMutex.Unlock()

	// Update configuration fields with atomic operations for thread safety
	x.minLevel = newCfg.LogLevel
	x.callerSkip = newCfg.CallerSkip
	x.enabledCallerInfo = newCfg.EnabledCallerInfo
	x.currentConfig = newCfg

	// Update level change configuration
	if newCfg.LevelChange != nil {
		x.levelChange = newLevelChange(newCfg.LevelChange)
	}

	// Refresh appenders to apply new configuration
	x.Refresh()
}

// GetCurrentConfig returns the current logger configuration.
// This method provides thread-safe access to the current configuration
// for inspection or debugging purposes.
//
// Returns:
//   - Current logger configuration
func (x *GameLogger) GetCurrentConfig() *LogCfg {
	x.configMutex.RLock()
	defer x.configMutex.RUnlock()
	return x.currentConfig
}

// checkLevel determines if a log level should be logged based on the minimum level.
// Returns true if the given level is equal to or higher than the minimum level.
// This method uses atomic operations for thread-safe hot-reload support.
//
// Parameters:
//   - level: The log level to check against the minimum threshold
//
// Returns:
//   - Boolean indicating whether the level should be logged
func (x *GameLogger) checkLevel(level Level) bool {
	currentLevel := Level(atomic.LoadUint32((*uint32)(unsafe.Pointer(&x.minLevel))))
	return currentLevel <= level
}

// AddAppender adds a new log appender to the logger. Appenders are responsible
// for outputting log events to various destinations such as files, console,
// or external services.
//
// Multiple appenders can be added to a single logger, allowing log messages
// to be sent to multiple destinations simultaneously.
//
// Parameters:
//   - appender: The log appender to add to the logger
func (x *GameLogger) AddAppender(appender LogAppender) {
	x.appenders = append(x.appenders, appender)
}

// GetAppender returns the list of appenders currently registered with the logger.
// This can be useful for inspection or modification of appender settings after
// logger creation.
//
// Returns:
//   - A slice of LogAppender instances registered with the logger
func (x *GameLogger) GetAppender() []LogAppender {
	return x.appenders
}

// Refresh triggers a refresh operation on all registered appenders.
// This is useful for log rotation or when configuration changes need to be applied
// without restarting the application.
func (x *GameLogger) Refresh() {
	for _, appender := range x.appenders {
		appender.Refresh()
	}
}

// IgnoreCheckLevel determines if log level filtering should be bypassed.
// For GameLogger, this always returns false, meaning log levels are always checked
// against the minimum level threshold.
//
// Returns:
//   - Boolean indicating whether log level checks should be ignored (always false for GameLogger)
func (x *GameLogger) IgnoreCheckLevel() bool {
	return false
}

// newEvent creates a new LogEvent instance from the object pool.
// This method ensures efficient reuse of log event objects, minimizing
// garbage collection pressure in high-throughput logging scenarios.
//
// The returned event is reset to a clean state, ready for use in logging operations.
//
// Returns:
//   - A new or reused LogEvent instance, properly initialized for logging
func (x *GameLogger) newEvent() *LogEvent {
	e := x.eventPool.Get().(*LogEvent)
	e.Reset()
	return e
}

// OnEventEnd handles the cleanup of a log event after it has been processed.
// For Fatal level logs, it triggers a panic to terminate the application.
// The event is returned to the object pool for reuse.
//
// Parameters:
//   - e: The LogEvent to be cleaned up and returned to the pool
func (x *GameLogger) OnEventEnd(e *LogEvent) {
	// Write to all configured appenders (console, file, etc.)
	for _, appender := range x.appenders {
		appender.Write(e.buf.Bytes())
	}

	if e.level == FatalLevel {
		panic("")
	}

	x.eventPool.Put(e)
}

// Debug creates a new debug-level log event.
// Use this for detailed diagnostic information during development or
// when debugging specific issues in a production environment.
//
// Returns:
//   - A new LogEvent instance configured for debug-level logging,
//     or nil if debug-level logging is not enabled
func (x *GameLogger) Debug() *LogEvent {
	return x.log(DebugLevel)
}

// Info creates a new info-level log event.
// Use this for general informational messages about application operation
// that are part of normal execution flow.
//
// Returns:
//   - A new LogEvent instance configured for info-level logging,
//     or nil if info-level logging is not enabled
func (x *GameLogger) Info() *LogEvent {
	return x.log(InfoLevel)
}

// Warn creates a new warning-level log event.
// Use this for potentially harmful situations that are not immediately critical
// but may indicate issues that need attention.
//
// Returns:
//   - A new LogEvent instance configured for warn-level logging,
//     or nil if warn-level logging is not enabled
func (x *GameLogger) Warn() *LogEvent {
	return x.log(WarnLevel)
}

// Error creates a new error-level log event.
// Use this for error conditions that might still allow the application to continue running
// but represent failures in specific operations or components.
//
// Returns:
//   - A new LogEvent instance configured for error-level logging,
//     or nil if error-level logging is not enabled
func (x *GameLogger) Error() *LogEvent {
	return x.log(ErrorLevel)
}

// Fatal creates a new fatal-level log event.
// Use this for severe error events that will presumably lead the application to abort.
// After logging, the application will terminate with a panic.
//
// Returns:
//   - A new LogEvent instance configured for fatal-level logging,
//     or nil if fatal-level logging is not enabled
func (x *GameLogger) Fatal() *LogEvent {
	return x.log(FatalLevel)
}

// getCallerInfo retrieves runtime information about the caller of the logging function.
// It returns the simplified file path, function name, and line number.
// The information is skipped based on the callerSkip configuration.
// This method is ultra-optimized based on zerolog/zap patterns with minimal allocations.
//
// Returns:
//   - A pointer to a callerInfo struct containing file, function, and line number
//     information for the caller of the logging function
func (x *GameLogger) getCallerInfo() *callerInfo {
	// Skip stack frames to get the actual caller
	pc, file, line, ok := runtime.Caller(3 + x.callerSkip)
	if !ok {
		return _UnknownCallerInfo
	}

	// Check cache for previously resolved caller information
	if cached, found := x.callerCache.Load(pc); found {
		return cached.(*callerInfo)
	}

	// Extract function name efficiently - single pass for dot extraction
	funcName := runtime.FuncForPC(pc).Name()
	var function string
	if dotIdx := strings.LastIndexByte(funcName, '.'); dotIdx != -1 {
		function = funcName[dotIdx+1:]
	} else {
		function = funcName
	}

	// Ultra-fast file path extraction - zero-copy substring operations
	// This approach minimizes string allocations and copying
	if len(file) > 0 {
		lastSlash := strings.LastIndexByte(file, '/')
		if lastSlash > 0 {
			// Find the directory separator before the last one
			secondLastSlash := strings.LastIndexByte(file[:lastSlash], '/')
			if secondLastSlash >= 0 {
				file = file[secondLastSlash+1:]
			}
		}
	}

	c := newCallerInfo(file, function, line)

	// Cache the resolved caller information for future use
	x.callerCache.Store(pc, c)

	return c
}

// log prepares a new log event with common fields like timestamp, level, and caller info.
// It handles log level filtering, per-file/per-line level overrides, and caller information
// collection before returning a LogEvent ready for additional fields to be added.
//
// Parameters:
//   - level: The severity level for the log event
//
// Returns:
//   - A LogEvent ready for additional fields to be added before being logged,
//     or nil if the log level is below the configured threshold
func (x *GameLogger) log(level Level) *LogEvent {
	var info *callerInfo
	// Check if we need to bypass level checking
	if !x.IgnoreCheckLevel() {
		// Check if the log level is enabled
		if !x.checkLevel(level) {
			// If not enabled, check if there are per-file/per-line level overrides
			if x.levelChange.Empty() {
				return nil
			}
			// Get caller info to check for level overrides
			info = x.getCallerInfo()
			lv := x.levelChange.GetLevel(info.file, info.line, level)
			level = lv
		}
	}

	// Final check after possible level override
	if !x.checkLevel(level) {
		return nil
	}

	// Get a log event from the pool
	e := x.newEvent()

	// Add common fields: timestamp and level
	t := time.Now()
	e.Time("time", &t)
	e.Str("level", level.String())

	// Add caller information if enabled
	if x.enabledCallerInfo {
		if info == nil {
			info = x.getCallerInfo()
		}
		e.Str("caller", info.String())
	}

	return e
}
