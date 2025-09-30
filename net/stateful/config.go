package stateful

import "fmt"

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

// GetName returns the configuration name for StatefulConfig
func (c *StatefulConfig) GetName() string {
	return "stateful"
}

// Validate validates the StatefulConfig parameters
func (c *StatefulConfig) Validate() error {
	if c.TickPeriodMillSec <= 0 {
		return fmt.Errorf("TickPeriodMillSec must be positive")
	}
	if c.GrHBTimeoutSec <= 0 {
		return fmt.Errorf("GrHBTimeoutSec must be positive")
	}
	if c.BroadcastQPS <= 0 {
		return fmt.Errorf("BroadcastQPS must be positive")
	}
	if c.MaxActorCount <= 0 {
		return fmt.Errorf("MaxActorCount must be positive")
	}
	if c.TaskChanSize <= 0 {
		return fmt.Errorf("TaskChanSize must be positive")
	}
	if c.GraceFulTimeoutSecond <= 0 {
		return fmt.Errorf("GraceFulTimeoutSecond must be positive")
	}
	return nil
}
