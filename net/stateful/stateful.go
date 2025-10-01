// Package stateful provides stateful message layer implementation for game servers.
// It manages actors with persistent state and handles message routing, actor lifecycle,
// and state migration in distributed game server environments.
package stateful

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/lcx/asura/config"
	"github.com/lcx/asura/log"
	"github.com/lcx/asura/metrics"
	"github.com/lcx/asura/net"
	"google.golang.org/protobuf/proto"
)

// MsgHandle defines the function signature for handling stateful messages.
// It processes incoming messages for actors and returns response messages with error codes.
type MsgHandle func(*HandleContext, Actor, proto.Message) (proto.Message, int32)

// MigrateMsgHandle defines the function signature for handling migration-related messages.
// It processes messages during actor state migration between servers.
type MigrateMsgHandle func(*HandleContext, proto.Message) (proto.Message, int32)

// StatefulMsgLayer implements the stateful message layer for managing game actors.
// It provides actor lifecycle management, message routing, and state persistence capabilities.
type StatefulMsgLayer struct {
	*StatefulConfig                     // Configuration for the stateful layer
	csTransport     net.CSTransport     // Client-server transport layer
	ssTransport     net.SSTransport     // Server-server transport layer
	actorMgr        *actorMgr           // Actor manager for lifecycle operations
	msgMgr          *net.MessageManager // Message manager for protocol handling
	lock            sync.RWMutex        // Protects configuration updates
}

// OnConfigChanged implements the ConfigChangeListener interface for StatefulMsgLayer.
// This method is called when the stateful configuration is updated in the config manager.
// It handles dynamic updates to stateful layer settings without requiring service restart.
func (layer *StatefulMsgLayer) OnConfigChanged(configName string, newConfig, oldConfig config.Config) error {
	if configName != "stateful" {
		return nil
	}

	newCfg, ok := newConfig.(*StatefulConfig)
	if !ok {
		return fmt.Errorf("invalid configuration type for StatefulMsgLayer")
	}

	// Validate the new configuration
	if err := newCfg.Validate(); err != nil {
		return fmt.Errorf("invalid stateful configuration: %w", err)
	}

	// Update configuration atomically
	layer.lock.Lock()
	defer layer.lock.Unlock()

	// Update configuration fields
	layer.StatefulConfig = newCfg

	log.Info().Str("configName", configName).Msg("Stateful message layer configuration updated successfully")
	return nil
}

// GetConfigName implements the ConfigChangeListener interface for StatefulMsgLayer.
// Returns the configuration name that this listener is interested in.
func (layer *StatefulMsgLayer) GetConfigName() string {
	return "stateful"
}

// NewMsgLayer creates a new stateful message layer instance.
// It initializes the layer with provided configuration, message manager, actor creator,
// and transport layers. If config is nil, it returns nil.
func NewMsgLayer(cfg *StatefulConfig, msgMgr *net.MessageManager, creator ActorCreator, cs net.CSTransport, ss net.SSTransport) (*StatefulMsgLayer, error) {
	if cfg == nil {
		return nil, errors.New("StatefulConfig cannot be nil, use NewMsgLayerWithConfigManager for dynamic configuration")
	}

	layer := &StatefulMsgLayer{
		StatefulConfig: cfg,
		csTransport:    cs,
		ssTransport:    ss,
		actorMgr:       newActorMgr(creator),
		msgMgr:         msgMgr,
		lock:           sync.RWMutex{},
	}

	return layer, nil
}

// NewMsgLayerWithConfigManager creates a stateful message layer that supports configuration hot-reload.
// This constructor initializes the layer with configuration from the config manager
// and registers it as a configuration change listener for dynamic updates.
func NewMsgLayerWithConfigManager(configManager config.ConfigManager, msgMgr *net.MessageManager, creator ActorCreator, cs net.CSTransport, ss net.SSTransport) (*StatefulMsgLayer, error) {
	if configManager == nil {
		return nil, errors.New("configManager cannot be nil")
	}

	// Load configuration from config manager
	cfg := &StatefulConfig{}
	if err := configManager.LoadConfig("stateful", cfg); err != nil {
		return nil, fmt.Errorf("failed to load stateful config: %w", err)
	}

	layer := &StatefulMsgLayer{
		StatefulConfig: cfg,
		csTransport:    cs,
		ssTransport:    ss,
		actorMgr:       newActorMgr(creator),
		msgMgr:         msgMgr,
		lock:           sync.RWMutex{},
	}

	// Register as configuration change listener
	configManager.AddChangeListener(layer)

	return layer, nil
}

// Init initializes the stateful message layer.
// Currently returns nil as no initialization is required.
func (layer *StatefulMsgLayer) Init() error {
	return nil
}

// Shutdown gracefully shuts down the stateful message layer.
// It notifies all actors to exit and waits for graceful termination.
func (layer *StatefulMsgLayer) Shutdown() {
	layer.actorMgr.shutdown()
}

// IsActorExist checks if an actor with the given ID exists in the system.
// Returns true if the actor is found, false otherwise.
func (layer *StatefulMsgLayer) IsActorExist(aid uint64) bool {
	_, ok := layer.actorMgr.getActorRuntime(aid)
	return ok
}

// GetActorCount returns the total number of active actors in the system.
func (layer *StatefulMsgLayer) GetActorCount() int {
	return layer.actorMgr.actorCount()
}

// GetMaxActorCount returns the maximum allowed number of actors.
func (layer *StatefulMsgLayer) GetMaxActorCount() int {
	return layer.MaxActorCount
}

// GetMigrateFeatSwitch returns the migration feature switch status.
// When true, actor state migration between servers is enabled.
func (layer *StatefulMsgLayer) GetMigrateFeatSwitch() bool {
	return layer.MigrateFeatSwitch
}

// NtfActorExit notifies all actors to exit gracefully.
// This is typically called during server shutdown.
func (layer *StatefulMsgLayer) NtfActorExit() {
	layer.actorMgr.ntfActorExit()
}

// DispatchCachedPkg dispatches a cached message package to the specified actor.
// It retrieves the actor runtime and posts the message for processing.
// Returns error if actor is not found or message processing fails.
func (layer *StatefulMsgLayer) DispatchCachedPkg(aid uint64, dd *net.DispatcherDelivery) error {
	var ar *actorRuntime
	ar, ok := layer.actorMgr.getActorRuntime(aid)
	if !ok {
		return fmt.Errorf("actorid:%d not find", aid)
	}
	hCtx := newHandleContext(aid, dd, ar.actor)
	return ar.postPkg(hCtx)
}

// OnRecvDispatcherPkg handles incoming dispatcher packages.
// It processes the package and logs any errors that occur.
func (layer *StatefulMsgLayer) OnRecvDispatcherPkg(dd *net.DispatcherDelivery) error {
	if err := layer.handleDispacherPkg(dd); err != nil {
		log.Error().Err(err).End()
	}
	return nil
}

// handleDispacherPkg processes dispatcher packages and routes them to appropriate actors.
// It validates the destination actor ID and delegates to handleActorPkg for processing.
func (layer *StatefulMsgLayer) handleDispacherPkg(delivery *net.DispatcherDelivery) (err error) {
	if delivery.Pkg.PkgHdr.DstActorID == 0 {
		log.Error().Msg("ActorID is 0")
	}

	return layer.handleActorPkg(delivery.Pkg.PkgHdr.DstActorID, delivery)
}

// handleActorPkg handles message packages for specific actors.
// It retrieves or creates the actor runtime, creates a handle context,
// and posts the message for processing.
func (layer *StatefulMsgLayer) handleActorPkg(aid uint64, delivery *net.DispatcherDelivery) (err error) {
	startTime := time.Now()
	metrics.IncrCounterWithGroup("net.stateful", "actor_message_total", 1)
	defer metrics.RecordStopwatchWithGroup("net.stateful", "actor_message_process_time", startTime)

	ar, ok := layer.actorMgr.getActorRuntime(aid)
	hCtx := &HandleContext{}
	if !ok {
		hCtx = newHandleContext(aid, delivery, ar.actor)
		ar, err = layer.tryGetActorRuntime(hCtx)
		if err != nil {
			metrics.IncrCounterWithDimGroup("net.stateful", "actor_error_total", 1, map[string]string{"error_type": "create_actor"})
			return err
		}
	}
	hCtx = newHandleContext(aid, delivery, ar.actor)
	if err = ar.postPkg(hCtx); err != nil {
		metrics.IncrCounterWithDimGroup("net.stateful", "actor_error_total", 1, map[string]string{"error_type": "post_message"})
	}

	return
}

// tryGetActorRuntime attempts to get an actor runtime, creating it if necessary.
// It first checks if the actor exists, then tries to decode the message body,
// and finally creates the actor if it doesn't exist.
func (layer *StatefulMsgLayer) tryGetActorRuntime(hCtx *HandleContext) (*actorRuntime, error) {
	a, ok := layer.actorMgr.getActorRuntime(hCtx.GetActorID())
	if ok {
		return a, nil
	}

	if _, err := hCtx.Pkg.DecodeBody(); err != nil {
		return nil, fmt.Errorf("ActorID:%d decodeBody error:%v", hCtx.GetActorID(), err)
	}

	return layer.actorMgr.createActor(hCtx.GetActorID())
}
