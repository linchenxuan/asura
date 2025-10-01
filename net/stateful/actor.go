package stateful

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/lcx/asura/log"
	"github.com/lcx/asura/metrics"
	"github.com/lcx/asura/net"
	"google.golang.org/protobuf/proto"
)

// ActorExitFunc is a callback function that gets executed when ExitActor is called.
// This function can be used to perform cleanup tasks before an actor exits.
type ActorExitFunc func()

// Actor is the interface that all stateful actors must implement.
// It extends the log.Logger interface to provide logging capabilities.
type Actor interface {
	log.Logger

	// Init is called when the actor is first created. It should perform any necessary initialization.
	Init() error
	// OnTick is called periodically based on the configured tick period.
	// It can be used for time-based operations like updating game state.
	OnTick() error
	// OnGracefulExit is called when the actor is about to exit gracefully.
	// It should clean up any resources held by the actor.
	OnGracefulExit()
	// OnMigrateFailRecover is called when an actor migration fails and needs to recover.
	// It should restore the actor to a consistent state.
	OnMigrateFailRecover() error
	// Save persists the actor's state to storage.
	Save()
	// ShowChange returns the changes made to the actor's state and a string identifier.
	ShowChange() (proto.Message, string)
	// SetActorExitFunc sets the function to be called when the actor exits.
	SetActorExitFunc(ActorExitFunc)
	// EncodeMigrateData encodes the actor's state into a byte slice for migration.
	EncodeMigrateData() ([]byte, error)
	// GetUpdateInternMS returns the interval in milliseconds at which the actor should be updated.
	GetUpdateInternMS() int
}

// ActorCreator is a function type for creating new actors.
// It takes a unique identifier (uid) and returns a new Actor instance and any error encountered.
type ActorCreator func(uid uint64) (Actor, error)

// ActorBase provides a base implementation of the Actor interface.
// It implements the SetActorExitFunc and ExitActor methods.
type ActorBase struct {
	exitFunc ActorExitFunc
}

// SetActorExitFunc sets the function to be called when ExitActor is invoked.
func (a *ActorBase) SetActorExitFunc(f ActorExitFunc) { a.exitFunc = f }

// ExitActor triggers the actor's exit function if it has been set.
// This method is typically called by the business layer to initiate actor shutdown.
func (a *ActorBase) ExitActor() {
	if a.exitFunc != nil {
		a.exitFunc()
	}
}

// actorRuntime represents the runtime environment for an actor.
// It manages the actor's lifecycle, message processing, and tick operations.
type actorRuntime struct {
	actor           Actor               // The actual actor instance
	actorID         uint64              // Unique identifier for the actor
	lastActive      int64               // Timestamp of the last time the actor was active
	lastFreqSaveSec int64               // Timestamp of the last frequent save operation
	closed          atomic.Bool         // Flag indicating whether the actor is closed
	pkgCtxChan      chan *HandleContext // Channel for incoming message contexts
	migrateChan     chan bool           // Channel for migration signals
	cancel          context.CancelFunc  // Function to cancel the actor's context
	ctx             context.Context     // Context for managing the actor's lifecycle

	msgLayer *StatefulMsgLayer // Reference to the message layer that owns this actor
}

// GetActor returns the underlying actor instance.
func (a *actorRuntime) GetActor() Actor {
	return a.actor
}

// newActorRuntime creates a new actor runtime environment for the given actor.
// It initializes channels, timestamps, and the context for the actor.
func newActorRuntime(msgLayer *StatefulMsgLayer, actorID uint64, actor Actor) *actorRuntime {
	ctx, cancel := context.WithCancel(context.Background())
	now := time.Now().Unix()
	a := &actorRuntime{
		actorID:         actorID,
		actor:           actor,
		pkgCtxChan:      make(chan *HandleContext, msgLayer.TaskChanSize),
		migrateChan:     make(chan bool, 1),
		lastActive:      now,
		lastFreqSaveSec: now,
		ctx:             ctx,
		cancel:          cancel,
		msgLayer:        msgLayer,
	}
	return a
}

// Cancel cancels the actor's context, initiating the shutdown process.
// It also logs a debug message indicating the cancellation.
func (a *actorRuntime) Cancel() {
	if a != nil && a.cancel != nil {
		a.actor.Debug().Msg("cancel")
		a.cancel()
	}
}

// initActor calls the actor's Init method to perform initialization.
// It returns any error encountered during initialization.
func (a *actorRuntime) initActor() error {
	return a.actor.Init()
}

// exitActor handles the graceful exit of the actor.
// It logs the exit event and cancels the actor's context.
func (a *actorRuntime) exitActor() {
	a.actor.Info().Msg("ExitActor")
	a.cancel()
}

// postPkg delivers a package context to the actor's message channel.
// If the actor is closed, it handles notification and request messages differently.
// It returns an error if the message channel is full or if there's an issue with delivery.
func (a *actorRuntime) postPkg(hCtx *HandleContext) error {
	if a.closed.Load() {
		// If it's a notification message, simply ignore it
		// For request messages, we need to send a response
		if !hCtx.ProtoInfo.IsReq() {
			hCtx.Info().Msg("actor is closed, ignore ntf msg")
			metrics.IncrCounterWithDimGroup("net.stateful", "actor_message_dropped_total", 1, map[string]string{"reason": "actor_closed", "message_type": "notification"})
			return nil
		}
		hCtx.Error().Msg("actor is closed, sendback req msg")
		metrics.IncrCounterWithDimGroup("net.stateful", "actor_message_dropped_total", 1, map[string]string{"reason": "actor_closed", "message_type": "request"})
		return nil
	}
	select {
	case a.pkgCtxChan <- hCtx:
		// 记录消息队列长度
		metrics.UpdateGaugeWithGroup("net.stateful", "actor_queue_length", metrics.Value(len(a.pkgCtxChan)))
		return nil
	default:
		msgType := "notification"
		if hCtx.ProtoInfo.IsReq() {
			msgType = "request"
		}
		metrics.IncrCounterWithDimGroup("net.stateful", "actor_message_dropped_total", 1, map[string]string{"reason": "queue_full",
			"message_type": msgType})

		return fmt.Errorf("actorID:%d pkgctx chan is full", a.actorID)
	}
}

// oneLoop processes a single iteration of the actor's main loop.
// It handles context cancellation, incoming messages, and tick operations.
// When the context is done, it closes the actor and processes remaining tasks.
func (a *actorRuntime) oneLoop() {
	t := time.NewTicker(time.Millisecond * time.Duration(a.msgLayer.TickPeriodMillSec))
	defer t.Stop()

	select {
	case <-a.ctx.Done():
		// 0. Close the channel
		a.closed.Store(true)

		// 1. Process remaining messages in the channel
		a.dealLeftTask()

		// 2. Execute graceful exit
		a.actor.OnGracefulExit()

	case hCtx := <-a.pkgCtxChan:
		a.lastActive = time.Now().Unix()
		if err := a.handlePkg(hCtx); err != nil {
			hCtx.Error().Err(err).Msg("handlePkg")
		}
	case <-t.C:
		// Execute tick operation
		a.tick()
	}
}

// dealLeftTask processes any remaining tasks in the actor's message channel.
// It logs the number of tasks to process and handles each one sequentially.
func (a *actorRuntime) dealLeftTask() {
	if len(a.pkgCtxChan) == 0 {
		return
	}

	a.actor.Info().Int("tasknum", len(a.pkgCtxChan)).Msg("left to do")
	for {
		select {
		case ctx := <-a.pkgCtxChan:
			if err := a.handlePkg(ctx); err != nil {
				ctx.Error().Err(err).Msg("handlePkg")
			}
		default:
			return
		}
	}
}

// runLoop starts the main processing loop for the actor.
// It initializes the actor and processes events until the actor is closed.
// It logs the start and exit of the task loop.
func (a *actorRuntime) runLoop() {
	a.actor.Info().Msg("stateful actor task loop start")
	defer a.actor.Info().Msg("stateful actor task loop exit")

	// Initialize the actor
	if err := a.initActor(); err != nil {
		a.actor.Warn().Err(err).Msg("actor OnInit err")
		return
	}

	for !a.closed.Load() {
		a.oneLoop()
	}
}

// tick executes the actor's OnTick method and handles periodic operations.
// It triggers frequent saves based on configuration and handles actor timeout if configured.
func (a *actorRuntime) tick() {
	startTime := time.Now()
	metrics.IncrCounterWithGroup("net.stateful", "actor_tick_total", 1)

	if err := a.actor.OnTick(); err != nil {
		a.actor.Error().Err(err).Msg("secureTask OnTick")
		metrics.IncrCounterWithGroup("net.stateful", "actor_tick_error_total", 1)
	}

	metrics.RecordStopwatchWithGroup("net.stateful", "actor_tick_process_time", startTime)

	now := time.Now().Unix()
	cfg := a.msgLayer.StatefulConfig
	if cfg.SaveCategory.FreqSaveSecond > 0 {
		if now >= a.lastFreqSaveSec+int64(cfg.SaveCategory.FreqSaveSecond) {
			saveStartTime := time.Now()
			a.actor.Save()
			metrics.RecordStopwatchWithGroup("net.stateful", "actor_save_time", saveStartTime)
			a.lastFreqSaveSec = now
		}
	}

	if cfg.ActorLifeSecond > 0 {
		if now >= a.lastActive+int64(cfg.ActorLifeSecond) {
			a.actor.Info().Int64("sec", now-a.lastActive).Msg("actor inactive for too long")
			metrics.IncrCounterWithGroup("net.stateful", "actor_timeout_total", 1)
			a.exitActor()
		}
	}
}

// handlePkg processes an incoming package context.
// It decodes the package body, finds the appropriate message handler,
// and executes it. For request messages, it sends back the response.
// It returns any error encountered during processing.
func (a *actorRuntime) handlePkg(hCtx *HandleContext) (err error) {
	startTime := time.Now()
	msgID := hCtx.ProtoInfo.MsgID
	isReq := hCtx.ProtoInfo.IsReq()

	// 记录处理消息的总数和类型分布
	dimensions := map[string]string{
		"msg_id":       msgID,
		"message_type": "notification",
	}
	if isReq {
		dimensions["message_type"] = "request"
	}
	metrics.IncrCounterWithDimGroup("net.stateful", "actor_message_process_total", 1, dimensions)

	// 使用defer确保无论函数从哪个路径返回，都会记录处理时间
	defer metrics.RecordStopwatchWithDimGroup("net.stateful", "actor_message_process_time", startTime, dimensions)

	body, err := hCtx.Pkg.DecodeBody()
	if err != nil {
		dimensions["error_type"] = "decode"
		metrics.IncrCounterWithDimGroup("net.stateful", "actor_message_error_total", 1, dimensions)
		return err
	}

	handle, ok := hCtx.ProtoInfo.GetMsgHandle().(MsgHandle)
	if !ok || handle == nil {
		dimensions["error_type"] = "no_handler"
		metrics.IncrCounterWithDimGroup("net.stateful", "actor_message_error_total", 1, dimensions)
		return errors.New("no handler")
	}

	hCtx.beforeSendPkgToClient = a.beforeSendPkgToClient

	res, code := handle(hCtx, a.actor, body)

	if hCtx.ProtoInfo.IsReq() {
		err = hCtx.sendBack(code, res)
		if err != nil {
			dimensions["error_type"] = "send_back"
			metrics.IncrCounterWithDimGroup("net.stateful", "actor_message_error_total", 1, dimensions)
		}
	}

	if code != int32(net.EAsuraRetCode_AsuraRetCodeOK) {
		dimensions["error_type"] = "business"
		dimensions["ret_code"] = fmt.Sprintf("%d", code)
		metrics.IncrCounterWithDimGroup("net.stateful", "actor_message_error_total", 1, dimensions)
		hCtx.Info().Str("MsgID", hCtx.ProtoInfo.MsgID).Int32("Retcode", code).End()
	} else {
		// 记录成功处理的消息
		metrics.IncrCounterWithDimGroup("net.stateful", "actor_message_success_total", 1, dimensions)
	}

	return
}

// beforeSendPkgToClient is a callback function executed before sending data to the client.
// It checks if the message type requires a save operation after handling
// and triggers the actor's Save method if configured to do so.
// It returns an error if the message ID is unknown.
func (a *actorRuntime) beforeSendPkgToClient(resPkg *net.TransSendPkg) error {
	pi, ok := a.msgLayer.msgMgr.GetProtoInfo(resPkg.PkgHdr.GetMsgID())
	if !ok {
		return fmt.Errorf("ActorID:%d msgid:%s unknown", a.actorID, resPkg.PkgHdr.GetMsgID())
	}

	if pi.IsRes() && a.msgLayer.SaveCategory.AfterHandleSave {
		a.actor.Save()
	}
	if pi.IsNtf() && a.msgLayer.SaveCategory.AfterHandleSave {
		a.actor.Save()
	}
	return nil
}
