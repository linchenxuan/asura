package stateful

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lcx/asura/log"
)

// actorMgr manages all actor instances in the system
// It provides thread-safe operations for creating, retrieving, and shutting down actors
// This is a core component of the stateful message processing system

type actorMgr struct {
	creator  ActorCreator             // Function to create new Actor instances when needed
	actorMap map[uint64]*actorRuntime // Mapping of actor IDs to their runtime instances
	lock     sync.RWMutex             // Read-write lock for thread-safe access to actorMap
	closed   int32                    // Atomic flag indicating if the manager is closed
	msgLayer *StatefulMsgLayer        // Reference to the message layer for configuration access
}

// _gracefulExitTick defines the interval for checking actor exit status during shutdown
const _gracefulExitTick = time.Millisecond * 10

// newActorMgr creates a new actor manager with the specified actor creator function
func newActorMgr(creator ActorCreator) *actorMgr {
	return &actorMgr{
		creator:  creator,
		actorMap: map[uint64]*actorRuntime{},
	}
}

// actorCount returns the current number of active actors
// This method also logs debug information about each actor
func (mgr *actorMgr) actorCount() int {
	for a := range mgr.actorMap {
		log.Debug().Uint64("actor", a).Msg("actorCount")
	}

	return len(mgr.actorMap)
}

// getActorRuntime retrieves an actor runtime instance by its ID
// It uses a read lock for thread-safe access without blocking write operations
func (mgr *actorMgr) getActorRuntime(id uint64) (a *actorRuntime, ok bool) {
	mgr.lock.RLock()
	defer mgr.lock.RUnlock()

	a, ok = mgr.actorMap[id]
	return
}

// createActor creates a new actor if it doesn't exist, or returns the existing one
// It enforces actor count limits and handles the creation process in a thread-safe manner
func (mgr *actorMgr) createActor(aid uint64) (*actorRuntime, error) {
	mgr.lock.Lock()
	defer mgr.lock.Unlock()

	// Check if actor already exists
	if a, ok := mgr.actorMap[aid]; ok {
		return a, nil
	}

	// Check if manager is closed
	if atomic.LoadInt32(&mgr.closed) != 0 {
		return nil, fmt.Errorf("actor:%d is closed", aid)
	}

	// Enforce actor count limit
	if len(mgr.actorMap) >= mgr.msgLayer.MaxActorCount {
		return nil, fmt.Errorf("actor:%d size:%d too much actor", aid, len(mgr.actorMap))
	}

	// Create new actor instance using the provided creator function
	actor, err := mgr.creator(aid)
	if err != nil {
		return nil, fmt.Errorf("actor:%d error:%v", aid, err)
	}

	// Create and initialize the actor runtime
	ar := newActorRuntime(mgr.msgLayer, aid, actor)
	actor.SetActorExitFunc(ar.exitActor)

	// Register the actor in the map
	mgr.actorMap[aid] = ar

	// Start the actor's main loop in a new goroutine
	go func() {
		defer delete(mgr.actorMap, aid) // Ensure actor is removed from map when done

		ar.runLoop()
	}()

	return ar, nil
}

// deleteActor removes an actor by cancelling its runtime
// Logs an error if the actor doesn't exist
func (mgr *actorMgr) deleteActor(id uint64) {
	if a, ok := mgr.getActorRuntime(id); ok {
		a.Cancel()
	} else {
		log.Error().Uint64("aid", id).Msg("ActorID has exited before")
	}
}

// getAllActors returns a slice of all active actor runtime instances
// Used for broadcasting messages to all actors
func (mgr *actorMgr) getAllActors() []*actorRuntime {
	mgr.lock.RLock()
	defer mgr.lock.RUnlock()
	ret := make([]*actorRuntime, len(mgr.actorMap))
	i := 0
	for _, v := range mgr.actorMap {
		ret[i] = v
		i++
	}
	return ret
}

// getAllActorsAID returns a slice of all active actor IDs
// Used for operations that need to know which actors exist without accessing their full runtime
func (mgr *actorMgr) getAllActorsAID() []uint64 {
	mgr.lock.RLock()
	defer mgr.lock.RUnlock()
	ret := make([]uint64, len(mgr.actorMap))
	i := 0
	for aid := range mgr.actorMap {
		ret[i] = aid
		i++
	}
	return ret
}

// shutdown initiates a graceful shutdown of all actors
// It waits for actors to exit within the configured timeout period
func (mgr *actorMgr) shutdown() {
	log.Info().Msg("actorMgr shutdown start")
	defer log.Info().Msg("actorMgr shutdown done")

	// Notify all actors to exit
	mgr.ntfActorExit()

	startTime := time.Now().Unix()
	t := time.NewTicker(_gracefulExitTick)
	defer t.Stop()

	// Wait for all actors to exit or until timeout
	for range t.C {
		mgr.lock.RLock()
		lens := len(mgr.actorMap)
		if lens == 0 {
			mgr.lock.RUnlock()
			log.Info().Msg("All gameobj are graceful exit")
			return
		}
		mgr.lock.RUnlock()
		now := time.Now().Unix()
		if now >= startTime+int64(mgr.msgLayer.GraceFulTimeoutSecond) {
			log.Error().Int("len", lens).Msg("Graceful Exit tiemout, there are still ActorCnt that no quit")
			return
		}
	}
}

// ntfActorExit notifies all actors to exit and marks the manager as closed
// This is a thread-safe operation that uses atomic operations for the closed flag
func (mgr *actorMgr) ntfActorExit() {
	mgr.lock.RLock()
	defer mgr.lock.RUnlock()
	atomic.StoreInt32(&mgr.closed, 1) // Atomically mark the manager as closed
	for _, a := range mgr.actorMap {
		a.exitActor() // Signal each actor to exit
	}
}
