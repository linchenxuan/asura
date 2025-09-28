// Package stateful provides stateful message layer implementation for game servers.
// It manages actors with persistent state and handles message routing, actor lifecycle,
// and state migration in distributed game server environments.
package stateful

import (
	"fmt"

	"github.com/lcx/asura/log"
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
}

// NewMsgLayer creates a new stateful message layer instance.
// It initializes the layer with provided configuration, message manager, actor creator,
// and transport layers. If config is nil, it uses default configuration.
func NewMsgLayer(cfg *StatefulConfig, msgMgr *net.MessageManager, creator ActorCreator, cs net.CSTransport, ss net.SSTransport) *StatefulMsgLayer {

	if cfg == nil {
		cfg = getDefaultConfig()
	}

	layer := &StatefulMsgLayer{
		StatefulConfig: cfg,
		csTransport:    cs,
		ssTransport:    ss,
		actorMgr:       newActorMgr(creator),
		msgMgr:         msgMgr,
	}

	return layer
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
	ar, ok := layer.actorMgr.getActorRuntime(aid)
	hCtx := &HandleContext{}
	if !ok {
		hCtx = newHandleContext(aid, delivery, ar.actor)
		ar, err = layer.tryGetActorRuntime(hCtx)
		if err != nil {
			return err
		}
	}
	hCtx = newHandleContext(aid, delivery, ar.actor)
	err = ar.postPkg(hCtx)
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
