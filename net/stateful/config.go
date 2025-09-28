package stateful

// ActorSaveCategory defines configuration for actor data persistence strategies.
// This struct controls how and when actor state data is saved to storage.
type ActorSaveCategory struct {
	AfterHandleSave bool   `mapstructure:"afterHandleSave"`
	FreqSave        bool   `mapstructure:"freqSave"`
	FreqSaveSecond  uint32 `mapstructure:"freqSaveSecond"`
}

// StatefulConfig contains configuration parameters for the stateful message layer.
// This struct defines various operational settings that control actor behavior,
// resource management, and system performance characteristics.
type StatefulConfig struct {
	SaveCategory          ActorSaveCategory `mapstructure:"saveCategory"`
	ActorLifeSecond       int               `mapstructure:"actorLifeSecond"`
	TickPeriodMillSec     int               `mapstructure:"tickPeriodMillSec"`
	GrHBTimeoutSec        int               `mapstructure:"grHBTimeoutSec"`
	BroadcastQPS          uint32            `mapstructure:"broadcastQPS"`
	BroadcastMaxWaitNum   int32             `mapstructure:"broadcastMaxWaitNum"`
	MaxActorCount         int               `mapstructure:"maxActorCount"`
	ActorNotCreateRetCode int32             `mapstructure:"actorNotCreateRetCode"`
	ActorMsgFilters       []string          `mapstructure:"actorMsgFilters"`
	MigrateFeatSwitch     bool              `mapstructure:"migrateFeatSwitch"`
	TaskChanSize          int               `mapstructure:"taskChanSize"`
	GraceFulTimeoutSecond int               `mapstructure:"graceFulTimeoutSecond"`
}

// getDefaultConfig returns a new StatefulConfig instance with default values.
// These defaults are chosen to provide reasonable performance characteristics
// for typical game server scenarios.
func getDefaultConfig() *StatefulConfig {
	return &StatefulConfig{
		TickPeriodMillSec:   20 * 1000, // 20 seconds
		GrHBTimeoutSec:      10,        // 10 seconds
		BroadcastQPS:        10000,     // 10,000 notifications per second
		BroadcastMaxWaitNum: 10,        // Queue up to 10 pending messages
		MaxActorCount:       5000,      // Allow up to 5000 concurrent actors
		MigrateFeatSwitch:   false,     // Migration feature disabled by default
	}
}
