package log

// LogCfg represents comprehensive logging configuration for high-performance game servers.
// It provides flexible configuration options for both synchronous and asynchronous logging,
// file rotation strategies, and output destinations suitable for production environments.
type LogCfg struct {
	// LogPath specifies the target log file path for file-based logging.
	// Supports relative and absolute paths with automatic directory creation.
	LogPath string `mapstructure:"path"`

	// LogLevel defines the minimum log level for filtering log entries.
	// Supports hot-reload without service restart for dynamic log level adjustment.
	// Valid levels: Trace, Debug, Info, Warn, Error, Fatal.
	LogLevel Level `mapstructure:"level"`

	// FileSplitMB determines the file rotation threshold in megabytes.
	// When log file exceeds this size, automatic rotation creates new files.
	// Supports hot-reload for runtime adjustment of rotation strategy.
	FileSplitMB int `mapstructure:"splitmb"`

	// FileSplitHour specifies the hour of day (0-23) for time-based file rotation.
	// Enables daily log rotation at specific times for operational convenience.
	FileSplitHour int `mapstructure:"splithour"`

	// IsAsync enables asynchronous log writing to prevent I/O blocking.
	// Recommended for high-throughput game servers to maintain low latency.
	IsAsync bool `mapstructure:"isasync"`

	// AsyncCacheSize limits the maximum buffered log entries in async mode.
	// Prevents memory overflow during traffic spikes or I/O slowdowns.
	// Default: 1024 entries when async mode is enabled.
	AsyncCacheSize int `mapstructure:"asynccachesize"`

	// AsyncWriteMillSec defines the async write interval in milliseconds.
	// Balances between write latency and batch efficiency for optimal performance.
	// Default: 200ms for reasonable trade-off between responsiveness and throughput.
	AsyncWriteMillSec int `mapstructure:"asyncwritemillsec"`

	// LevelChangeMin enables dynamic minimum log level adjustment.
	// Allows runtime log level changes for debugging or performance tuning.
	LevelChangeMin int `mapstructure:"levelchangemin"`

	// CallerSkip specifies the number of stack frames to skip for caller information.
	// Useful for wrapper functions or middleware layers in complex applications.
	CallerSkip int `mapstructure:"callerSkip"`

	// FileAppender enables file-based logging output.
	// Primary logging destination for persistent storage and analysis.
	FileAppender bool `mapstructure:"fileAppender"`

	// ConsoleAppender enables console (stdout) logging output.
	// Convenient for development and containerized environments.
	ConsoleAppender bool `mapstructure:"consoleAppender"`

	// LevelChange enables fine-grained log level control for specific code locations.
	// Allows runtime adjustment of logging verbosity without service restart.
	// Each entry maps a file path and line number to a specific log level.
	// Designed for debugging critical game server components in production.
	LevelChange []LevelChangeEntry `mapstructure:"levelChange"`

	// ActorWhiteList defines the list of player/actor IDs that bypass log level filtering.
	// Enables targeted debugging for specific high-priority players or test accounts.
	// Supports hot-reload for dynamic addition/removal of debug targets.
	// Example: [123456789, 987654321, 555666777]
	ActorWhiteList []uint64 `mapstructure:"actorWhiteList"`

	// actorWhiteListSet is an internal cache for O(1) whitelist lookups.
	// Populated automatically from ActorWhiteList during configuration initialization.
	// Not intended for direct configuration - use ActorWhiteList instead.
	actorWhiteListSet map[uint64]struct{} `mapstructure:"-"`

	// ActorFileLog enables logging to actor-specific log files.
	// When enabled, ActorLogger will output to both the original log file and actor-specific files.
	// When disabled, ActorLogger will only output to the original log file.
	ActorFileLog bool `mapstructure:"actorFileLog"`

	EnabledCallerInfo bool `mapstructure:"enabledCallerInfo"`
}

// IsInWhiteList checks if an actor ID exists in the whitelist with O(1) complexity.
// Returns true if the actor is whitelisted for unrestricted logging.
func (cfg *LogCfg) IsInWhiteList(actorID uint64) bool {
	if len(cfg.actorWhiteListSet) == 0 && len(cfg.ActorWhiteList) != 0 {
		cfg.actorWhiteListSet = make(map[uint64]struct{}, len(cfg.ActorWhiteList))
		for _, id := range cfg.ActorWhiteList {
			cfg.actorWhiteListSet[id] = struct{}{}
		}
	}

	_, exists := cfg.actorWhiteListSet[actorID]
	return exists
}

var _defaultCfg = &LogCfg{
	LogPath:         "./asura.log",
	LogLevel:        DebugLevel, // Default log level
	FileSplitMB:     50,
	FileSplitHour:   0,
	IsAsync:         true,
	CallerSkip:      1,
	FileAppender:    true,
	ConsoleAppender: true,
}

func getDefaultCfg() *LogCfg {
	return _defaultCfg
}
