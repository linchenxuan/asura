package log

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
)

// ActorLogger provides specialized logging functionality for game actors with dual output capabilities.
// It extends the base GameLogger to support both standard logging (to the original log file)
// and actor-specific logging (to a dedicated actor log file) with runtime toggle control.
//
// Key features:
// - Always outputs to the original log file (ensuring no log loss)
// - Conditionally outputs to actor-specific log file
// - Runtime toggle capability for actor file logging
// - Maintains all standard GameLogger functionality
//
// The logger ensures that critical actor events are always logged to the main system log
// while providing the flexibility to enable detailed actor-specific logging only when needed.
type ActorLogger struct {
	*GameLogger
	actorID     uint64
	inWhiteList bool
}

// NewActorLogger creates a new ActorLogger instance for a specific actor.
// The logger always outputs to the original log file and conditionally outputs
// to an actor-specific log file based on the ActorFileLog configuration and
// runtime toggle settings.
//
// Parameters:
//   - cfg: Base logging configuration
//   - actorID: Unique identifier for the actor
//
// Returns:
//   - A new ActorLogger instance configured for the specified actor
func NewActorLogger(cfg *LogCfg, actorID uint64) *ActorLogger {
	if cfg == nil {
		cfg = getDefaultCfg()
	}

	// Create base logger with all standard appenders
	logger := &GameLogger{
		minLevel:          cfg.LogLevel,
		callerSkip:        cfg.CallerSkip,
		levelChange:       newLevelChange(cfg.LevelChange),
		enabledCallerInfo: cfg.EnabledCallerInfo,
	}

	actorLogger := &ActorLogger{
		GameLogger:  logger,
		actorID:     actorID,
		inWhiteList: cfg.IsInWhiteList(actorID),
	}

	// Initialize object pool for LogEvent instances
	logger.eventPool = &sync.Pool{
		New: func() any {
			return newEvent(logger)
		},
	}

	// Always add console appender if enabled
	if cfg.ConsoleAppender {
		logger.AddAppender(NewConsoleAppender())
	}

	// Always add original file appender if enabled - this ensures output to original file
	if cfg.FileAppender {
		logger.AddAppender(NewFileAppender(cfg, logger))
	}

	if cfg.ActorFileLog {
		// Create a copy of config for actor-specific logging
		actorCfgCopy := *cfg
		ext := filepath.Ext(actorCfgCopy.LogPath)
		base := strings.TrimSuffix(actorCfgCopy.LogPath, ext)
		actorCfgCopy.LogPath = fmt.Sprintf("%s_%d%s", base, actorID, ext)

		actorLogger.AddAppender(NewFileAppender(&actorCfgCopy, actorLogger))
	}

	return actorLogger
}

// log creates a new log event with actor-specific context.
// It automatically adds the "actor" field with the actor ID to all log entries
// for correlation and tracking across the system.
//
// Parameters:
//   - level: Log severity level for the event
//
// Returns:
//   - A new LogEvent instance with the actor ID already populated
func (x *ActorLogger) log(level Level) *LogEvent {
	logEvent := x.GameLogger.log(level)
	if logEvent == nil {
		return nil
	}

	return logEvent.Uint64("actor", x.actorID)
}

// IgnoreCheckLevel determines if log level filtering should be bypassed for this actor.
// It returns true for whitelisted actors, enabling verbose debugging regardless of the global log level.
// This allows detailed logging for specific actors even when the global log level is set to a higher threshold.
//
// Returns:
//   - Boolean indicating whether log level checks should be ignored
func (x *ActorLogger) IgnoreCheckLevel() bool {
	return x.inWhiteList
}

// Debug creates a new debug-level log event for actor-specific debugging.
// Use this method for detailed diagnostic information during development
// or targeted troubleshooting of specific actors in production.
//
// Returns:
//   - A new LogEvent instance configured for debug-level logging
func (x *ActorLogger) Debug() *LogEvent {
	return x.log(DebugLevel)
}

// Info creates a new info-level log event for actor operation tracking.
// Use this method for general informational messages about actor state changes,
// such as login/logout events, inventory updates, or quest progress.
//
// Returns:
//   - A new LogEvent instance configured for info-level logging
func (x *ActorLogger) Info() *LogEvent {
	return x.log(InfoLevel)
}

// Warn creates a new warning-level log event for potentially problematic actor operations.
// Use this method for situations that might require attention but don't prevent
// the actor from continuing normal operation, such as deprecated API usage
// or minor configuration issues.
//
// Returns:
//   - A new LogEvent instance configured for warn-level logging
func (x *ActorLogger) Warn() *LogEvent {
	return x.log(WarnLevel)
}

// Error creates a new error-level log event for actor-specific failures.
// Use this method for significant problems that affect the actor's operation
// but don't necessarily crash the application, such as failed database transactions
// or network connectivity issues.
//
// Returns:
//   - A new LogEvent instance configured for error-level logging
func (x *ActorLogger) Error() *LogEvent {
	return x.log(ErrorLevel)
}

// Fatal creates a new fatal-level log event for critical actor failures.
// Use this method for severe errors that prevent the actor from continuing operation.
// After logging, the application will terminate with a panic.
//
// Returns:
//   - A new LogEvent instance configured for fatal-level logging
func (x *ActorLogger) Fatal() *LogEvent {
	return x.log(FatalLevel)
}
